package posh

import (
	"container/list"
	"errors"
	"fmt"
	"log" // TODO: no
	"regexp"
	"strconv"
	"strings"
)

var embeddedPosh *regexp.Regexp = regexp.MustCompile(`\(\(\s*(.*?)\s*\)\)`)
var pathName *regexp.Regexp = regexp.MustCompile("^[a-zA-Z0-9_]+$")

type Context []map[string]Node

type Spice struct {
	Stub Node

	path    []string
	context Context
}

type PoshNode struct {
	Node

	Expression Expression

	path    []string
	context Context
}

func (s *Spice) Flow(root Node) (Node, bool) {
	return s.flow(root, []string{}, []map[string]Node{})
}

func CheckResolved(root Node) error {
	switch root.(type) {
	case map[string]Node:
		for _, val := range root.(map[string]Node) {
			err := CheckResolved(val)
			if err != nil {
				return err
			}
		}

	case []Node:
		for _, val := range root.([]Node) {
			err := CheckResolved(val)
			if err != nil {
				return err
			}
		}

	case *PoshNode:
		posh := root.(*PoshNode)

		return errors.New(fmt.Sprintf("could not resolve: %#v\n", posh.Expression))

	case string:

	default:
		return errors.New(fmt.Sprintf("unknown node type: %#v\n", root))
	}

	return nil
}

func (s *Spice) flow(root Node, path []string, context Context) (Node, bool) {
	switch root.(type) {
	case map[string]Node:
		return s.flowMap(root.(map[string]Node), path, context)

	case []Node:
		return s.flowList(root.([]Node), path, context)

	case string:
		return s.flowScalar(root.(string), path, context)

	case *PoshNode:
		posh := root.(*PoshNode)
		evaluated := posh.Expression.Evaluate(context, s.Stub)

		if evaluated != nil {
			posh.Node = evaluated
			return posh.Node, true
		}

		return posh, false

	case int, bool:
		return root, false

	default:
		panic(fmt.Sprintf("unknown node type during flow: %#v\n", root))
	}
}

func (s *Spice) flowMap(root map[string]Node, path []string, context Context) (Node, bool) {
	newMap := make(map[string]Node)

	didFlow := false

	for key, val := range root {
		flowedVal, didFlowVal := s.flow(val, append(path, key), append(context, root))
		newMap[key] = flowedVal

		if didFlowVal {
			didFlow = true
		}
	}

	return Node(newMap), didFlow
}

func (s *Spice) flowList(root []Node, path []string, context Context) (Node, bool) {
	newList := []Node{}

	didFlow := false

	for _, val := range root {
		entryPath := path

		switch val.(type) {
		case map[string]Node:
			nameNode := val.(map[string]Node)["name"]

			name, ok := nameNode.(string)
			if ok && pathName.MatchString(name) {
				entryPath = append(path, name)
			}
		}

		flowedVal, didFlowVal := s.flow(val, entryPath, context)
		if didFlowVal {
			didFlow = true
		}

		newList = append(newList, flowedVal)
	}

	return Node(newList), didFlow
}

func (s *Spice) flowScalar(root string, path []string, context Context) (Node, bool) {
	sub := embeddedPosh.FindStringSubmatch(root)
	if sub == nil {
		return root, false
	}

	poshContent := sub[1]

	posh := &Posh{Buffer: poshContent}
	posh.Init()

	if err := posh.Parse(); err != nil {
		log.Fatal(err)
	}

	result := compileTokens(posh, path, context)
	if result == nil {
		return root, false
	}

	return result, true
}

type ExprStack struct {
	list.List
}

func (s *ExprStack) Pop() Expression {
	front := s.Front()
	if front == nil {
		return nil
	}

	s.Remove(front)

	return front.Value.(Expression)
}

func (s *ExprStack) Push(expr Expression) {
	s.PushFront(expr)
}

func (s *ExprStack) PopSeq() *SeqExpr {
	expr := s.Pop()

	seq, ok := expr.(*SeqExpr)
	if !ok {
		seq = &SeqExpr{
			Expressions: []Expression{expr},
		}
	}

	return seq
}

func compileTokens(posh *Posh, path []string, context Context) Node {
	exprStack := &ExprStack{}

	afterComma := false
	for token := range posh.Tokens() {
		contents := posh.Buffer[token.begin:token.end]

		switch token.Rule {
		case RulePosh:
			expr := exprStack.Pop()

			return &PoshNode{
				Expression: expr,

				path:    path,
				context: context,
			}
		case RuleAuto:
			exprStack.Push(&AutoExpr{path})
		case RuleMerge:
			exprStack.Push(&MergeExpr{path})
		case RuleReference:
			exprStack.Push(&ReferenceExpr{strings.Split(contents, ".")})
		case RuleInteger:
			val, err := strconv.Atoi(contents)
			if err != nil {
				panic(err)
			}

			exprStack.Push(&IntegerExpr{val})
		case RuleBoolean:
			exprStack.Push(&BooleanExpr{contents == "true"})
		case RuleString:
			// strip quotes (TODO: hack)
			exprStack.Push(&StringExpr{contents[1 : len(contents)-1]})
		case RuleOr:
			rhs := exprStack.Pop()
			lhs := exprStack.Pop()

			exprStack.Push(&OrExpr{A: lhs, B: rhs})
		case RuleConcatenation:
			rhs := exprStack.Pop()
			lhs := exprStack.Pop()

			exprStack.Push(&ConcatenationExpr{A: lhs, B: rhs})
		case RuleAddition:
			rhs := exprStack.Pop()
			lhs := exprStack.Pop()

			exprStack.Push(&AdditionExpr{A: lhs, B: rhs})
		case RuleSubtraction:
			rhs := exprStack.Pop()
			lhs := exprStack.Pop()

			exprStack.Push(&SubtractionExpr{A: lhs, B: rhs})
		case RuleCall:
			seq := exprStack.PopSeq()

			function, ok := exprStack.Pop().(*FunctionExpr)
			if !ok {
				panic("non-function in call")
			}

			exprStack.Push(&CallExpr{
				Name:      function.Name,
				Arguments: seq.Expressions,
			})
		case RuleName:
			exprStack.Push(&FunctionExpr{Name: contents})
		case RuleList:
			seq := exprStack.PopSeq()
			exprStack.Push(&ListExpr{seq.Expressions})
		case RuleComma:
			afterComma = true
		case RuleArguments:
			// no-op (wrapped by Call)
		case RuleContents:
			// no-op (wrapped by List)
		case RuleGrouped:
			// no-op
		case RuleLevel0, RuleLevel1, RuleLevel2:
		case RuleExpression:
			if afterComma {
				expr := exprStack.Pop()
				seq := exprStack.PopSeq()

				exprStack.Push(&SeqExpr{
					Expressions: append(seq.Expressions, expr),
				})

				afterComma = false
			}
		case Rulews:
		default:
			log.Fatalln("unhandled:", Rul3s[token.Rule])
		}
	}

	panic("unreachable")
}
