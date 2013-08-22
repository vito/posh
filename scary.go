package posh

import (
	"fmt"
)

type Node interface{}

func Sanitize(root interface{}) Node {
	switch root.(type) {
	case map[interface{}]interface{}:
		sanitized := map[string]Node{}

		for key, val := range root.(map[interface{}]interface{}) {
			str, ok := key.(string)
			if !ok {
				panic("NO")
			}

			sanitized[str] = Sanitize(val)
		}

		return Node(sanitized)

	case []interface{}:
		sanitized := []Node{}

		for _, val := range root.([]interface{}) {
			sanitized = append(sanitized, Sanitize(val))
		}

		return Node(sanitized)

	case string:
		return Node(root.(string))

	case []byte:
		return Node(string(root.([]byte)))

	// TODO
	case int, bool:
		return Node(fmt.Sprintf("%v", root))

	default:
		panic(fmt.Sprintf("unknown type during sanitization: %#v\n", root))
	}
}
