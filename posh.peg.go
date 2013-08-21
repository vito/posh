package posh

import (
	/*"bytes"*/
	"fmt"
	"math"
	"sort"
	"strconv"
)

const END_SYMBOL byte = 0

/* The rule types inferred from the grammar are below. */
type Rule uint8

const (
	RuleUnknown Rule = iota
	RulePosh
	RuleExpression
	RuleLevel2
	RuleOr
	RuleLevel1
	RuleAddition
	RuleSubtraction
	RuleLevel0
	RuleGrouped
	RuleCall
	RuleArguments
	RuleName
	RuleComma
	RuleInteger
	RuleString
	RuleList
	RuleContents
	RuleMerge
	RuleAuto
	RuleReference
	Rulews

	RulePre_
	Rule_In_
	Rule_Suf
)

var Rul3s = [...]string{
	"Unknown",
	"Posh",
	"Expression",
	"Level2",
	"Or",
	"Level1",
	"Addition",
	"Subtraction",
	"Level0",
	"Grouped",
	"Call",
	"Arguments",
	"Name",
	"Comma",
	"Integer",
	"String",
	"List",
	"Contents",
	"Merge",
	"Auto",
	"Reference",
	"ws",

	"Pre_",
	"_In_",
	"_Suf",
}

type TokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule Rule, begin, end, next, depth int)
	Expand(index int) TokenTree
	Tokens() <-chan token32
	Error() []token32
	trim(length int)
}

/* ${@} bit structure for abstract syntax tree */
type token16 struct {
	Rule
	begin, end, next int16
}

func (t *token16) isZero() bool {
	return t.Rule == RuleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token16) isParentOf(u token16) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token16) GetToken32() token32 {
	return token32{Rule: t.Rule, begin: int32(t.begin), end: int32(t.end), next: int32(t.next)}
}

func (t *token16) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", Rul3s[t.Rule], t.begin, t.end, t.next)
}

type tokens16 struct {
	tree    []token16
	ordered [][]token16
}

func (t *tokens16) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens16) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens16) Order() [][]token16 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int16, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.Rule == RuleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token16, len(depths)), make([]token16, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = int16(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type State16 struct {
	token16
	depths []int16
	leaf   bool
}

func (t *tokens16) PreOrder() (<-chan State16, [][]token16) {
	s, ordered := make(chan State16, 6), t.Order()
	go func() {
		var states [8]State16
		for i, _ := range states {
			states[i].depths = make([]int16, len(ordered))
		}
		depths, state, depth := make([]int16, len(ordered)), 0, 1
		write := func(t token16, leaf bool) {
			S := states[state]
			state, S.Rule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.Rule, t.begin, t.end, int16(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token16 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token16{Rule: Rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token16{Rule: RulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.Rule != RuleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.Rule != RuleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token16{Rule: Rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens16) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", Rul3s[token.Rule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", Rul3s[token.Rule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", Rul3s[token.Rule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens16) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", Rul3s[token.Rule], strconv.Quote(buffer[token.begin:token.end]))
	}
}

func (t *tokens16) Add(rule Rule, begin, end, depth, index int) {
	t.tree[index] = token16{Rule: rule, begin: int16(begin), end: int16(end), next: int16(depth)}
}

func (t *tokens16) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.GetToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens16) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].GetToken32()
		}
	}
	return tokens
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	Rule
	begin, end, next int32
}

func (t *token32) isZero() bool {
	return t.Rule == RuleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) GetToken32() token32 {
	return token32{Rule: t.Rule, begin: int32(t.begin), end: int32(t.end), next: int32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", Rul3s[t.Rule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.Rule == RuleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = int32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type State32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) PreOrder() (<-chan State32, [][]token32) {
	s, ordered := make(chan State32, 6), t.Order()
	go func() {
		var states [8]State32
		for i, _ := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.Rule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.Rule, t.begin, t.end, int32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{Rule: Rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{Rule: RulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.Rule != RuleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.Rule != RuleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{Rule: Rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", Rul3s[token.Rule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", Rul3s[token.Rule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", Rul3s[ordered[i][depths[i]-1].Rule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", Rul3s[token.Rule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", Rul3s[token.Rule], strconv.Quote(buffer[token.begin:token.end]))
	}
}

func (t *tokens32) Add(rule Rule, begin, end, depth, index int) {
	t.tree[index] = token32{Rule: rule, begin: int32(begin), end: int32(end), next: int32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.GetToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].GetToken32()
		}
	}
	return tokens
}

func (t *tokens16) Expand(index int) TokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		for i, v := range tree {
			expanded[i] = v.GetToken32()
		}
		return &tokens32{tree: expanded}
	}
	return nil
}

func (t *tokens32) Expand(index int) TokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type Posh struct {
	Buffer string
	rules  [22]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	TokenTree
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer string, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer[0:] {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p *Posh
}

func (e *parseError) Error() string {
	tokens, error := e.p.TokenTree.Error(), "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.Buffer, positions)
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf("parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n",
			Rul3s[token.Rule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			/*strconv.Quote(*/ e.p.Buffer[begin:end] /*)*/)
	}

	return error
}

func (p *Posh) PrintSyntaxTree() {
	p.TokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *Posh) Highlighter() {
	p.TokenTree.PrintSyntax()
}

func (p *Posh) Init() {
	if p.Buffer[len(p.Buffer)-1] != END_SYMBOL {
		p.Buffer = p.Buffer + string(END_SYMBOL)
	}

	var tree TokenTree = &tokens16{tree: make([]token16, math.MaxInt16)}
	position, depth, tokenIndex, buffer, rules := 0, 0, 0, p.Buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.TokenTree = tree
		if matches {
			p.TokenTree.trim(tokenIndex)
			return nil
		}
		return &parseError{p}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule Rule, begin int) {
		if t := tree.Expand(tokenIndex); t != nil {
			tree = t
		}
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
	}

	matchDot := func() bool {
		if buffer[position] != END_SYMBOL {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	rules = [...]func() bool{
		nil,
		/* 0 Posh <- <(Expression !.)> */
		func() bool {
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
				if !rules[RuleExpression]() {
					goto l0
				}
				{
					position2, tokenIndex2, depth2 := position, tokenIndex, depth
					if !matchDot() {
						goto l2
					}
					goto l0
				l2:
					position, tokenIndex, depth = position2, tokenIndex2, depth2
				}
				depth--
				add(RulePosh, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 Expression <- <Level2> */
		func() bool {
			position3, tokenIndex3, depth3 := position, tokenIndex, depth
			{
				position4 := position
				depth++
				if !rules[RuleLevel2]() {
					goto l3
				}
				depth--
				add(RuleExpression, position4)
			}
			return true
		l3:
			position, tokenIndex, depth = position3, tokenIndex3, depth3
			return false
		},
		/* 2 Level2 <- <(Or / Level1)> */
		func() bool {
			position5, tokenIndex5, depth5 := position, tokenIndex, depth
			{
				position6 := position
				depth++
				{
					position7, tokenIndex7, depth7 := position, tokenIndex, depth
					if !rules[RuleOr]() {
						goto l8
					}
					goto l7
				l8:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[RuleLevel1]() {
						goto l5
					}
				}
			l7:
				depth--
				add(RuleLevel2, position6)
			}
			return true
		l5:
			position, tokenIndex, depth = position5, tokenIndex5, depth5
			return false
		},
		/* 3 Or <- <(Level1 ws ('|' '|') ws Expression)> */
		func() bool {
			position9, tokenIndex9, depth9 := position, tokenIndex, depth
			{
				position10 := position
				depth++
				if !rules[RuleLevel1]() {
					goto l9
				}
				if !rules[Rulews]() {
					goto l9
				}
				if buffer[position] != '|' {
					goto l9
				}
				position++
				if buffer[position] != '|' {
					goto l9
				}
				position++
				if !rules[Rulews]() {
					goto l9
				}
				if !rules[RuleExpression]() {
					goto l9
				}
				depth--
				add(RuleOr, position10)
			}
			return true
		l9:
			position, tokenIndex, depth = position9, tokenIndex9, depth9
			return false
		},
		/* 4 Level1 <- <(Addition / Subtraction / Level0)> */
		func() bool {
			position11, tokenIndex11, depth11 := position, tokenIndex, depth
			{
				position12 := position
				depth++
				{
					position13, tokenIndex13, depth13 := position, tokenIndex, depth
					if !rules[RuleAddition]() {
						goto l14
					}
					goto l13
				l14:
					position, tokenIndex, depth = position13, tokenIndex13, depth13
					if !rules[RuleSubtraction]() {
						goto l15
					}
					goto l13
				l15:
					position, tokenIndex, depth = position13, tokenIndex13, depth13
					if !rules[RuleLevel0]() {
						goto l11
					}
				}
			l13:
				depth--
				add(RuleLevel1, position12)
			}
			return true
		l11:
			position, tokenIndex, depth = position11, tokenIndex11, depth11
			return false
		},
		/* 5 Addition <- <(Level0 ws '+' ws Level1)> */
		func() bool {
			position16, tokenIndex16, depth16 := position, tokenIndex, depth
			{
				position17 := position
				depth++
				if !rules[RuleLevel0]() {
					goto l16
				}
				if !rules[Rulews]() {
					goto l16
				}
				if buffer[position] != '+' {
					goto l16
				}
				position++
				if !rules[Rulews]() {
					goto l16
				}
				if !rules[RuleLevel1]() {
					goto l16
				}
				depth--
				add(RuleAddition, position17)
			}
			return true
		l16:
			position, tokenIndex, depth = position16, tokenIndex16, depth16
			return false
		},
		/* 6 Subtraction <- <(Level0 ws '-' ws Level1)> */
		func() bool {
			position18, tokenIndex18, depth18 := position, tokenIndex, depth
			{
				position19 := position
				depth++
				if !rules[RuleLevel0]() {
					goto l18
				}
				if !rules[Rulews]() {
					goto l18
				}
				if buffer[position] != '-' {
					goto l18
				}
				position++
				if !rules[Rulews]() {
					goto l18
				}
				if !rules[RuleLevel1]() {
					goto l18
				}
				depth--
				add(RuleSubtraction, position19)
			}
			return true
		l18:
			position, tokenIndex, depth = position18, tokenIndex18, depth18
			return false
		},
		/* 7 Level0 <- <(Grouped / Call / String / Integer / List / Merge / Auto / Reference)> */
		func() bool {
			position20, tokenIndex20, depth20 := position, tokenIndex, depth
			{
				position21 := position
				depth++
				{
					position22, tokenIndex22, depth22 := position, tokenIndex, depth
					if !rules[RuleGrouped]() {
						goto l23
					}
					goto l22
				l23:
					position, tokenIndex, depth = position22, tokenIndex22, depth22
					if !rules[RuleCall]() {
						goto l24
					}
					goto l22
				l24:
					position, tokenIndex, depth = position22, tokenIndex22, depth22
					if !rules[RuleString]() {
						goto l25
					}
					goto l22
				l25:
					position, tokenIndex, depth = position22, tokenIndex22, depth22
					if !rules[RuleInteger]() {
						goto l26
					}
					goto l22
				l26:
					position, tokenIndex, depth = position22, tokenIndex22, depth22
					if !rules[RuleList]() {
						goto l27
					}
					goto l22
				l27:
					position, tokenIndex, depth = position22, tokenIndex22, depth22
					if !rules[RuleMerge]() {
						goto l28
					}
					goto l22
				l28:
					position, tokenIndex, depth = position22, tokenIndex22, depth22
					if !rules[RuleAuto]() {
						goto l29
					}
					goto l22
				l29:
					position, tokenIndex, depth = position22, tokenIndex22, depth22
					if !rules[RuleReference]() {
						goto l20
					}
				}
			l22:
				depth--
				add(RuleLevel0, position21)
			}
			return true
		l20:
			position, tokenIndex, depth = position20, tokenIndex20, depth20
			return false
		},
		/* 8 Grouped <- <('(' Expression ')')> */
		func() bool {
			position30, tokenIndex30, depth30 := position, tokenIndex, depth
			{
				position31 := position
				depth++
				if buffer[position] != '(' {
					goto l30
				}
				position++
				if !rules[RuleExpression]() {
					goto l30
				}
				if buffer[position] != ')' {
					goto l30
				}
				position++
				depth--
				add(RuleGrouped, position31)
			}
			return true
		l30:
			position, tokenIndex, depth = position30, tokenIndex30, depth30
			return false
		},
		/* 9 Call <- <(Name '(' Arguments ')')> */
		func() bool {
			position32, tokenIndex32, depth32 := position, tokenIndex, depth
			{
				position33 := position
				depth++
				if !rules[RuleName]() {
					goto l32
				}
				if buffer[position] != '(' {
					goto l32
				}
				position++
				if !rules[RuleArguments]() {
					goto l32
				}
				if buffer[position] != ')' {
					goto l32
				}
				position++
				depth--
				add(RuleCall, position33)
			}
			return true
		l32:
			position, tokenIndex, depth = position32, tokenIndex32, depth32
			return false
		},
		/* 10 Arguments <- <(Expression (Comma ws Expression)*)> */
		func() bool {
			position34, tokenIndex34, depth34 := position, tokenIndex, depth
			{
				position35 := position
				depth++
				if !rules[RuleExpression]() {
					goto l34
				}
			l36:
				{
					position37, tokenIndex37, depth37 := position, tokenIndex, depth
					if !rules[RuleComma]() {
						goto l37
					}
					if !rules[Rulews]() {
						goto l37
					}
					if !rules[RuleExpression]() {
						goto l37
					}
					goto l36
				l37:
					position, tokenIndex, depth = position37, tokenIndex37, depth37
				}
				depth--
				add(RuleArguments, position35)
			}
			return true
		l34:
			position, tokenIndex, depth = position34, tokenIndex34, depth34
			return false
		},
		/* 11 Name <- <([a-z] / [A-Z] / [0-9] / '_')+> */
		func() bool {
			position38, tokenIndex38, depth38 := position, tokenIndex, depth
			{
				position39 := position
				depth++
				{
					position42, tokenIndex42, depth42 := position, tokenIndex, depth
					if c := buffer[position]; c < 'a' || c > 'z' {
						goto l43
					}
					position++
					goto l42
				l43:
					position, tokenIndex, depth = position42, tokenIndex42, depth42
					if c := buffer[position]; c < 'A' || c > 'Z' {
						goto l44
					}
					position++
					goto l42
				l44:
					position, tokenIndex, depth = position42, tokenIndex42, depth42
					if c := buffer[position]; c < '0' || c > '9' {
						goto l45
					}
					position++
					goto l42
				l45:
					position, tokenIndex, depth = position42, tokenIndex42, depth42
					if buffer[position] != '_' {
						goto l38
					}
					position++
				}
			l42:
			l40:
				{
					position41, tokenIndex41, depth41 := position, tokenIndex, depth
					{
						position46, tokenIndex46, depth46 := position, tokenIndex, depth
						if c := buffer[position]; c < 'a' || c > 'z' {
							goto l47
						}
						position++
						goto l46
					l47:
						position, tokenIndex, depth = position46, tokenIndex46, depth46
						if c := buffer[position]; c < 'A' || c > 'Z' {
							goto l48
						}
						position++
						goto l46
					l48:
						position, tokenIndex, depth = position46, tokenIndex46, depth46
						if c := buffer[position]; c < '0' || c > '9' {
							goto l49
						}
						position++
						goto l46
					l49:
						position, tokenIndex, depth = position46, tokenIndex46, depth46
						if buffer[position] != '_' {
							goto l41
						}
						position++
					}
				l46:
					goto l40
				l41:
					position, tokenIndex, depth = position41, tokenIndex41, depth41
				}
				depth--
				add(RuleName, position39)
			}
			return true
		l38:
			position, tokenIndex, depth = position38, tokenIndex38, depth38
			return false
		},
		/* 12 Comma <- <','> */
		func() bool {
			position50, tokenIndex50, depth50 := position, tokenIndex, depth
			{
				position51 := position
				depth++
				if buffer[position] != ',' {
					goto l50
				}
				position++
				depth--
				add(RuleComma, position51)
			}
			return true
		l50:
			position, tokenIndex, depth = position50, tokenIndex50, depth50
			return false
		},
		/* 13 Integer <- <([0-9] / '_')+> */
		func() bool {
			position52, tokenIndex52, depth52 := position, tokenIndex, depth
			{
				position53 := position
				depth++
				{
					position56, tokenIndex56, depth56 := position, tokenIndex, depth
					if c := buffer[position]; c < '0' || c > '9' {
						goto l57
					}
					position++
					goto l56
				l57:
					position, tokenIndex, depth = position56, tokenIndex56, depth56
					if buffer[position] != '_' {
						goto l52
					}
					position++
				}
			l56:
			l54:
				{
					position55, tokenIndex55, depth55 := position, tokenIndex, depth
					{
						position58, tokenIndex58, depth58 := position, tokenIndex, depth
						if c := buffer[position]; c < '0' || c > '9' {
							goto l59
						}
						position++
						goto l58
					l59:
						position, tokenIndex, depth = position58, tokenIndex58, depth58
						if buffer[position] != '_' {
							goto l55
						}
						position++
					}
				l58:
					goto l54
				l55:
					position, tokenIndex, depth = position55, tokenIndex55, depth55
				}
				depth--
				add(RuleInteger, position53)
			}
			return true
		l52:
			position, tokenIndex, depth = position52, tokenIndex52, depth52
			return false
		},
		/* 14 String <- <('"' (!'"' .)* '"')> */
		func() bool {
			position60, tokenIndex60, depth60 := position, tokenIndex, depth
			{
				position61 := position
				depth++
				if buffer[position] != '"' {
					goto l60
				}
				position++
			l62:
				{
					position63, tokenIndex63, depth63 := position, tokenIndex, depth
					{
						position64, tokenIndex64, depth64 := position, tokenIndex, depth
						if buffer[position] != '"' {
							goto l64
						}
						position++
						goto l63
					l64:
						position, tokenIndex, depth = position64, tokenIndex64, depth64
					}
					if !matchDot() {
						goto l63
					}
					goto l62
				l63:
					position, tokenIndex, depth = position63, tokenIndex63, depth63
				}
				if buffer[position] != '"' {
					goto l60
				}
				position++
				depth--
				add(RuleString, position61)
			}
			return true
		l60:
			position, tokenIndex, depth = position60, tokenIndex60, depth60
			return false
		},
		/* 15 List <- <('[' Contents ']')> */
		func() bool {
			position65, tokenIndex65, depth65 := position, tokenIndex, depth
			{
				position66 := position
				depth++
				if buffer[position] != '[' {
					goto l65
				}
				position++
				if !rules[RuleContents]() {
					goto l65
				}
				if buffer[position] != ']' {
					goto l65
				}
				position++
				depth--
				add(RuleList, position66)
			}
			return true
		l65:
			position, tokenIndex, depth = position65, tokenIndex65, depth65
			return false
		},
		/* 16 Contents <- <(Expression (Comma ws Expression)*)> */
		func() bool {
			position67, tokenIndex67, depth67 := position, tokenIndex, depth
			{
				position68 := position
				depth++
				if !rules[RuleExpression]() {
					goto l67
				}
			l69:
				{
					position70, tokenIndex70, depth70 := position, tokenIndex, depth
					if !rules[RuleComma]() {
						goto l70
					}
					if !rules[Rulews]() {
						goto l70
					}
					if !rules[RuleExpression]() {
						goto l70
					}
					goto l69
				l70:
					position, tokenIndex, depth = position70, tokenIndex70, depth70
				}
				depth--
				add(RuleContents, position68)
			}
			return true
		l67:
			position, tokenIndex, depth = position67, tokenIndex67, depth67
			return false
		},
		/* 17 Merge <- <('m' 'e' 'r' 'g' 'e')> */
		func() bool {
			position71, tokenIndex71, depth71 := position, tokenIndex, depth
			{
				position72 := position
				depth++
				if buffer[position] != 'm' {
					goto l71
				}
				position++
				if buffer[position] != 'e' {
					goto l71
				}
				position++
				if buffer[position] != 'r' {
					goto l71
				}
				position++
				if buffer[position] != 'g' {
					goto l71
				}
				position++
				if buffer[position] != 'e' {
					goto l71
				}
				position++
				depth--
				add(RuleMerge, position72)
			}
			return true
		l71:
			position, tokenIndex, depth = position71, tokenIndex71, depth71
			return false
		},
		/* 18 Auto <- <('a' 'u' 't' 'o')> */
		func() bool {
			position73, tokenIndex73, depth73 := position, tokenIndex, depth
			{
				position74 := position
				depth++
				if buffer[position] != 'a' {
					goto l73
				}
				position++
				if buffer[position] != 'u' {
					goto l73
				}
				position++
				if buffer[position] != 't' {
					goto l73
				}
				position++
				if buffer[position] != 'o' {
					goto l73
				}
				position++
				depth--
				add(RuleAuto, position74)
			}
			return true
		l73:
			position, tokenIndex, depth = position73, tokenIndex73, depth73
			return false
		},
		/* 19 Reference <- <(([a-z] / [A-Z] / [0-9] / '_')+ ('.' ([a-z] / [A-Z] / [0-9] / '_')+)*)> */
		func() bool {
			position75, tokenIndex75, depth75 := position, tokenIndex, depth
			{
				position76 := position
				depth++
				{
					position79, tokenIndex79, depth79 := position, tokenIndex, depth
					if c := buffer[position]; c < 'a' || c > 'z' {
						goto l80
					}
					position++
					goto l79
				l80:
					position, tokenIndex, depth = position79, tokenIndex79, depth79
					if c := buffer[position]; c < 'A' || c > 'Z' {
						goto l81
					}
					position++
					goto l79
				l81:
					position, tokenIndex, depth = position79, tokenIndex79, depth79
					if c := buffer[position]; c < '0' || c > '9' {
						goto l82
					}
					position++
					goto l79
				l82:
					position, tokenIndex, depth = position79, tokenIndex79, depth79
					if buffer[position] != '_' {
						goto l75
					}
					position++
				}
			l79:
			l77:
				{
					position78, tokenIndex78, depth78 := position, tokenIndex, depth
					{
						position83, tokenIndex83, depth83 := position, tokenIndex, depth
						if c := buffer[position]; c < 'a' || c > 'z' {
							goto l84
						}
						position++
						goto l83
					l84:
						position, tokenIndex, depth = position83, tokenIndex83, depth83
						if c := buffer[position]; c < 'A' || c > 'Z' {
							goto l85
						}
						position++
						goto l83
					l85:
						position, tokenIndex, depth = position83, tokenIndex83, depth83
						if c := buffer[position]; c < '0' || c > '9' {
							goto l86
						}
						position++
						goto l83
					l86:
						position, tokenIndex, depth = position83, tokenIndex83, depth83
						if buffer[position] != '_' {
							goto l78
						}
						position++
					}
				l83:
					goto l77
				l78:
					position, tokenIndex, depth = position78, tokenIndex78, depth78
				}
			l87:
				{
					position88, tokenIndex88, depth88 := position, tokenIndex, depth
					if buffer[position] != '.' {
						goto l88
					}
					position++
					{
						position91, tokenIndex91, depth91 := position, tokenIndex, depth
						if c := buffer[position]; c < 'a' || c > 'z' {
							goto l92
						}
						position++
						goto l91
					l92:
						position, tokenIndex, depth = position91, tokenIndex91, depth91
						if c := buffer[position]; c < 'A' || c > 'Z' {
							goto l93
						}
						position++
						goto l91
					l93:
						position, tokenIndex, depth = position91, tokenIndex91, depth91
						if c := buffer[position]; c < '0' || c > '9' {
							goto l94
						}
						position++
						goto l91
					l94:
						position, tokenIndex, depth = position91, tokenIndex91, depth91
						if buffer[position] != '_' {
							goto l88
						}
						position++
					}
				l91:
				l89:
					{
						position90, tokenIndex90, depth90 := position, tokenIndex, depth
						{
							position95, tokenIndex95, depth95 := position, tokenIndex, depth
							if c := buffer[position]; c < 'a' || c > 'z' {
								goto l96
							}
							position++
							goto l95
						l96:
							position, tokenIndex, depth = position95, tokenIndex95, depth95
							if c := buffer[position]; c < 'A' || c > 'Z' {
								goto l97
							}
							position++
							goto l95
						l97:
							position, tokenIndex, depth = position95, tokenIndex95, depth95
							if c := buffer[position]; c < '0' || c > '9' {
								goto l98
							}
							position++
							goto l95
						l98:
							position, tokenIndex, depth = position95, tokenIndex95, depth95
							if buffer[position] != '_' {
								goto l90
							}
							position++
						}
					l95:
						goto l89
					l90:
						position, tokenIndex, depth = position90, tokenIndex90, depth90
					}
					goto l87
				l88:
					position, tokenIndex, depth = position88, tokenIndex88, depth88
				}
				depth--
				add(RuleReference, position76)
			}
			return true
		l75:
			position, tokenIndex, depth = position75, tokenIndex75, depth75
			return false
		},
		/* 20 ws <- <(' ' / '\t' / '\n' / '\r')*> */
		func() bool {
			{
				position100 := position
				depth++
			l101:
				{
					position102, tokenIndex102, depth102 := position, tokenIndex, depth
					{
						position103, tokenIndex103, depth103 := position, tokenIndex, depth
						if buffer[position] != ' ' {
							goto l104
						}
						position++
						goto l103
					l104:
						position, tokenIndex, depth = position103, tokenIndex103, depth103
						if buffer[position] != '\t' {
							goto l105
						}
						position++
						goto l103
					l105:
						position, tokenIndex, depth = position103, tokenIndex103, depth103
						if buffer[position] != '\n' {
							goto l106
						}
						position++
						goto l103
					l106:
						position, tokenIndex, depth = position103, tokenIndex103, depth103
						if buffer[position] != '\r' {
							goto l102
						}
						position++
					}
				l103:
					goto l101
				l102:
					position, tokenIndex, depth = position102, tokenIndex102, depth102
				}
				depth--
				add(Rulews, position100)
			}
			return true
		},
	}
	p.rules = rules
}
