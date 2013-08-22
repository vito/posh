package posh

import (
	"errors"
	"fmt"
	"log" // TODO: no
	"regexp"
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
