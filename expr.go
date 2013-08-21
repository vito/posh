package posh

import (
	"fmt"
	"strconv"

	"github.com/kylelemons/go-gypsy/yaml"
)

type Expression interface {
	Evaluate(context []yaml.Map, stub yaml.Node) yaml.Node
}

type AutoExpr struct {
	Path []string
}

type MergeExpr struct {
	Path []string
}

type ReferenceExpr struct {
	Path []string
}

type IntegerExpr struct {
	Value int
}

type StringExpr struct {
	Value string
}

type OrExpr struct {
	A Expression
	B Expression
}

type ConcatenationExpr struct {
	A Expression
	B Expression
}

type AdditionExpr struct {
	A Expression
	B Expression
}

type SubtractionExpr struct {
	A Expression
	B Expression
}

type FunctionExpr struct {
	Name string
}

type SeqExpr struct {
	Expressions []Expression
}

type ListExpr struct {
	Contents []Expression
}

type CallExpr struct {
	Name      string
	Arguments []Expression
}

func (e *AutoExpr) Evaluate([]yaml.Map, yaml.Node) yaml.Node {
	return yaml.Scalar("TODO Auto")
}

func (e *MergeExpr) Evaluate(context []yaml.Map, stub yaml.Node) yaml.Node {
	return findInPath(e.Path, stub)
}

func (e *ReferenceExpr) Evaluate(context []yaml.Map, stub yaml.Node) yaml.Node {
	root, found := resolveSymbol(e.Path[0], context...)
	if !found {
		return yaml.Scalar("TODO: reference not found: " + e.Path[0])
	}

	return findInPath(e.Path[1:], root)
}

func (e *IntegerExpr) Evaluate([]yaml.Map, yaml.Node) yaml.Node {
	return yaml.Scalar(fmt.Sprintf("%d", e.Value))
}

func (e *StringExpr) Evaluate([]yaml.Map, yaml.Node) yaml.Node {
	return yaml.Scalar(e.Value)
}

func (e *OrExpr) Evaluate(context []yaml.Map, stub yaml.Node) yaml.Node {
	a := e.A.Evaluate(context, stub)
	if a != nil {
		return a
	}

	return e.B.Evaluate(context, stub)
}

func (e *ConcatenationExpr) Evaluate(context []yaml.Map, stub yaml.Node) yaml.Node {
	a := e.A.Evaluate(context, stub)
	b := e.B.Evaluate(context, stub)

	ascalar, ok := scalarFrom(a)
	if !ok {
		fmt.Printf("NOT SCALAR: %#v\n", a)
		return nil
	}

	bscalar, ok := scalarFrom(b)
	if !ok {
		return nil
	}

	return yaml.Scalar(string(ascalar) + string(bscalar))
}

func (e *AdditionExpr) Evaluate(context []yaml.Map, stub yaml.Node) yaml.Node {
	a := e.A.Evaluate(context, stub)
	b := e.B.Evaluate(context, stub)

	ascalar, ok := scalarFrom(a)
	if !ok {
		fmt.Printf("NOT SCALAR: %#v\n", a)
		return nil
	}

	bscalar, ok := scalarFrom(b)
	if !ok {
		return nil
	}

	aint, err := strconv.Atoi(string(ascalar))
	if err != nil {
		return nil
	}

	bint, err := strconv.Atoi(string(bscalar))
	if err != nil {
		return nil
	}

	return yaml.Scalar(fmt.Sprintf("%d", aint+bint))
}

func (e *SubtractionExpr) Evaluate(context []yaml.Map, stub yaml.Node) yaml.Node {
	a := e.A.Evaluate(context, stub)
	b := e.B.Evaluate(context, stub)

	ascalar, ok := scalarFrom(a)
	if !ok {
		fmt.Printf("NOT SCALAR: %#v\n", a)
		return nil
	}

	bscalar, ok := scalarFrom(b)
	if !ok {
		return nil
	}

	aint, err := strconv.Atoi(string(ascalar))
	if err != nil {
		return nil
	}

	bint, err := strconv.Atoi(string(bscalar))
	if err != nil {
		return nil
	}

	return yaml.Scalar(fmt.Sprintf("%d", aint-bint))
}

func (e *SeqExpr) Evaluate([]yaml.Map, yaml.Node) yaml.Node {
	return yaml.Scalar("TODO Seq")
}

func (e *FunctionExpr) Evaluate([]yaml.Map, yaml.Node) yaml.Node {
	return yaml.Scalar("TODO Function")
}

func (e *CallExpr) Evaluate([]yaml.Map, yaml.Node) yaml.Node {
	return yaml.Scalar("TODO Call")
}

func (e *ListExpr) Evaluate(context []yaml.Map, stub yaml.Node) yaml.Node {
	var nodes []yaml.Node

	for _, sub := range e.Contents {
		nodes = append(nodes, sub.Evaluate(context, stub))
	}

	return yaml.List(nodes)
}

func scalarFrom(node yaml.Node) (yaml.Scalar, bool) {
	switch node.(type) {
	case yaml.Scalar:
		return node.(yaml.Scalar), true
	case *PoshNode:
		return scalarFrom(node.(*PoshNode).Node)
	default:
		return yaml.Scalar(""), false
	}
}

func findInPath(path []string, root yaml.Node) yaml.Node {
	here := root

	for _, step := range path {
		if here == nil {
			return nil
		}

		var found bool

		here, found = nextStep(step, here)
		if !found {
			return nil
		}
	}

	return here
}

func nextStep(step string, here yaml.Node) (yaml.Node, bool) {
	found := false
	switch here.(type) {
	case yaml.Map:
		found = true
		here = here.(yaml.Map).Key(step)
	case yaml.List:
		for _, val := range []yaml.Node(here.(yaml.List)) {
			switch val.(type) {
			case yaml.Map:
				name := val.(yaml.Map).Key("name")

				switch name.(type) {
				case yaml.Scalar:
					if string(name.(yaml.Scalar)) == step {
						found = true
						here = val
					}
				}
			}
		}
	case *PoshNode:
		here, found = nextStep(step, here.(*PoshNode).Node)
	default:
	}

	if !found {
		// fmt.Printf("failed on step %#v in %#v\n", step, here)
		return nil, false
	}

	return here, true
}

func resolveSymbol(name string, context ...yaml.Map) (yaml.Node, bool) {
	for _, ctx := range context {
		val := ctx.Key(name)
		if val != nil {
			return val, true
		}
	}

	return nil, false
}
