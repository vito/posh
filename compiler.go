package posh

import (
	"container/list"
	"log"
	"strconv"
	"strings"
)

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
