package posh

import (
	"container/list"
	"errors"
	"fmt"
	"log" // TODO: no
	"regexp"
	"strconv"
	"strings"

	"github.com/kylelemons/go-gypsy/yaml"
)

var embeddedPosh *regexp.Regexp = regexp.MustCompile(`{{\s*(.*?)\s*}}`)
var pathName *regexp.Regexp = regexp.MustCompile("^[a-zA-Z0-9_]+$")

type Spice struct {
	Stub yaml.Node

	path    []string
	context []yaml.Map
}

type PoshNode struct {
	yaml.Node

	Expression Expression

	path    []string
	context []yaml.Map
}

func (s *Spice) Flow(root yaml.Node) (yaml.Node, bool) {
	return s.flow(root, []string{}, []yaml.Map{})
}

func CheckResolved(root yaml.Node) error {
	switch root.(type) {
	case yaml.Map:
		for _, val := range map[string]yaml.Node(root.(yaml.Map)) {
			err := CheckResolved(val)
			if err != nil {
				return err
			}
		}

	case yaml.List:
		for _, val := range []yaml.Node(root.(yaml.List)) {
			err := CheckResolved(val)
			if err != nil {
				return err
			}
		}

	case yaml.Scalar:

	case *PoshNode:
		posh := root.(*PoshNode)

		return errors.New(fmt.Sprintf("could not resolve: %#v\n", posh.Expression))

	default:
		panic("unknown node type")
	}

	return nil
}

func (s *Spice) flow(root yaml.Node, path []string, context []yaml.Map) (yaml.Node, bool) {
	switch root.(type) {
	case yaml.Map:
		return s.flowMap(root.(yaml.Map), path, context)

	case yaml.List:
		return s.flowList(root.(yaml.List), path, context)

	case yaml.Scalar:
		return s.flowScalar(root.(yaml.Scalar), path, context)

	case *PoshNode:
		posh := root.(*PoshNode)
		evaluated := posh.Expression.Evaluate(context, s.Stub)

		if evaluated != nil {
			posh.Node = evaluated
			return posh.Node, true
		}

		return posh, false

	default:
		panic("unknown node type")
	}
}

func (s *Spice) flowMap(root yaml.Map, path []string, context []yaml.Map) (yaml.Node, bool) {
	newMap := make(map[string]yaml.Node)

	didFlow := false

	for key, val := range map[string]yaml.Node(root) {
		flowedVal, didFlowVal := s.flow(val, append(path, key), append(context, root))
		newMap[key] = flowedVal

		if didFlowVal {
			didFlow = true
		}
	}

	return yaml.Map(newMap), didFlow
}

func (s *Spice) flowList(root yaml.List, path []string, context []yaml.Map) (yaml.Node, bool) {
	newList := []yaml.Node{}

	didFlow := false

	for _, val := range []yaml.Node(root) {
		entryPath := path

		switch val.(type) {
		case yaml.Map:
			nameNode := val.(yaml.Map).Key("name")
			name, ok := nameNode.(yaml.Scalar)

			if ok && pathName.MatchString(string(name)) {
				entryPath = append(path, string(name))
			}
		}

		flowedVal, didFlowVal := s.flow(val, entryPath, context)
		if didFlowVal {
			didFlow = true
		}

		newList = append(newList, flowedVal)
	}

	return yaml.List(newList), didFlow
}

func (s *Spice) flowScalar(root yaml.Scalar, path []string, context []yaml.Map) (yaml.Node, bool) {
	sub := embeddedPosh.FindStringSubmatch(string(root))
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

func compileTokens(posh *Posh, path []string, context []yaml.Map) yaml.Node {
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

func autoFor(path []string) string {
	if len(path) == 3 && path[0] == "resource_pools" && path[2] == "size" {
		resourcePool := path[1]
		return fmt.Sprintf(
			`find("jobs").select { |attrs| attrs["resource_pool"] == %#v }.collect { |attrs| attrs["instances"] }.inject(&:+)`,
			resourcePool,
		)
	}

	return "UNKNOWN_AUTO"
}
