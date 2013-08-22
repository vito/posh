package posh

import (
	"fmt"
	"strconv"
)

type Expression interface {
	Evaluate(context Context, stub Node) Node
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

type BooleanExpr struct {
	Value bool
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

func (e *AutoExpr) Evaluate(context Context, stub Node) Node {
	if len(e.Path) == 3 && e.Path[0] == "resource_pools" && e.Path[2] == "size" {
		size := 0

		jobs, found := resolveSymbol("jobs", context)
		if !found {
			return nil
		}

		jobsList, ok := jobs.([]Node)
		if !ok {
			return nil
		}

		for _, job := range []Node(jobsList) {
			attrs, ok := job.(map[string]Node)
			if !ok {
				continue
			}

			resourcePool, ok := attrs["resource_pool"]
			if !ok {
				continue
			}

			poolName, ok := scalarFrom(resourcePool)
			if !ok {
				continue
			}

			if poolName != e.Path[1] {
				continue
			}

			instances, ok := attrs["instances"]
			if !ok {
				return nil
			}

			instanceCount, ok := scalarFrom(instances)
			if !ok {
				return nil
			}

			count, err := strconv.Atoi(instanceCount)
			if err != nil {
				return nil
			}

			size += count
		}

		return Node(fmt.Sprintf("%d", size))
	}

	return nil
}

func (e *MergeExpr) Evaluate(context Context, stub Node) Node {
	return findInPath(e.Path, stub)
}

func (e *ReferenceExpr) Evaluate(context Context, stub Node) Node {
	root, found := resolveSymbol(e.Path[0], context)
	if !found {
		return nil
	}

	return findInPath(e.Path[1:], root)
}

func (e *BooleanExpr) Evaluate(Context, Node) Node {
	return Node(fmt.Sprintf("%v", e.Value))
}

func (e *IntegerExpr) Evaluate(Context, Node) Node {
	return Node(fmt.Sprintf("%d", e.Value))
}

func (e *StringExpr) Evaluate(Context, Node) Node {
	return Node(e.Value)
}

func (e *OrExpr) Evaluate(context Context, stub Node) Node {
	a := e.A.Evaluate(context, stub)
	if a != nil {
		return a
	}

	return e.B.Evaluate(context, stub)
}

func (e *ConcatenationExpr) Evaluate(context Context, stub Node) Node {
	a := e.A.Evaluate(context, stub)
	b := e.B.Evaluate(context, stub)

	ascalar, ok := scalarFrom(a)
	if !ok {
		return nil
	}

	bscalar, ok := scalarFrom(b)
	if !ok {
		return nil
	}

	return Node(ascalar + bscalar)
}

func (e *AdditionExpr) Evaluate(context Context, stub Node) Node {
	a := e.A.Evaluate(context, stub)
	b := e.B.Evaluate(context, stub)

	ascalar, ok := scalarFrom(a)
	if !ok {
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

	return Node(fmt.Sprintf("%d", aint+bint))
}

func (e *SubtractionExpr) Evaluate(context Context, stub Node) Node {
	a := e.A.Evaluate(context, stub)
	b := e.B.Evaluate(context, stub)

	ascalar, ok := scalarFrom(a)
	if !ok {
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

	return Node(fmt.Sprintf("%d", aint-bint))
}

func (e *SeqExpr) Evaluate(Context, Node) Node {
	return Node("TODO Seq")
}

func (e *FunctionExpr) Evaluate(Context, Node) Node {
	return Node("TODO Function")
}

func (e *CallExpr) Evaluate(Context, Node) Node {
	return Node("TODO Call")
}

func (e *ListExpr) Evaluate(context Context, stub Node) Node {
	var nodes []Node

	for _, sub := range e.Contents {
		nodes = append(nodes, sub.Evaluate(context, stub))
	}

	return Node(nodes)
}

func scalarFrom(node Node) (string, bool) {
	switch node.(type) {
	case string:
		return node.(string), true
	case *PoshNode:
		return scalarFrom(node.(*PoshNode).Node)
	default:
		return "", false
	}
}

func findInPath(path []string, root Node) Node {
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

func nextStep(step string, here Node) (Node, bool) {
	found := false
	switch here.(type) {
	case map[string]Node:
		found = true
		here = here.(map[string]Node)[step]
	case []Node:
		for _, val := range here.([]Node) {
			switch val.(type) {
			case map[string]Node:
				name := val.(map[string]Node)["name"]

				switch name.(type) {
				case string:
					if name.(string) == step {
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
		fmt.Printf("failed on step %#v in %#v\n", step, here)
		return nil, false
	}

	return here, true
}

func resolveSymbol(name string, context Context) (Node, bool) {
	for _, ctx := range context {
		val := ctx[name]
		if val != nil {
			return val, true
		}
	}

	return nil, false
}
