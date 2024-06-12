package flatten

import (
	"fmt"
	"reflect"
)

type stackFrame struct {
	prefix      string
	nested      interface{}
	setPrevious *interface{}
}

func (s stackFrame) setPrev(v interface{}) {
	rv := reflect.ValueOf(s.setPrevious)
	rv.Set(reflect.ValueOf(v))
}

func flatFast(prefix string, nested interface{}) (M, error) {
	//var (
	//	order    []string
	//	visitAll func(items []string)
	//)
	//
	//seen := make(map[string]bool)
	//visitAll = func(items []string) {
	//	for _, item := range items {
	//		if !seen[item] {
	//			seen[item] = true
	//			visitAll(m[item])
	//			order = append(order, item)
	//		}
	//	}
	//}

	stack := []stackFrame{{prefix, nested}}
	result := M{}

	// outer:
	for len(stack) > 0 {
		// Pop from stack
		currentFrame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		prefix := currentFrame.prefix
		nested := currentFrame.nested

		switch n := nested.(type) {
		case M:
			for key, value := range n {
				if isOpKey(key) {
					if !isKeyValidOperator(key) {
						return nil, fmt.Errorf("%s starts with a $ and is not a valid operator", key)
					}

					if key == "$or" || key == "$and" {
						switch a := value.(type) {
						case []interface{}:
							for i := range a {
								stack = append(stack, stackFrame{"", a[i]})
							}

							if len(a) > 0 {
								result[key] = a
								continue
							}
						case []M:
							for i := range a {
								stack = append(stack, stackFrame{"", a[i]})
							}
							if len(a) > 0 {
								result[key] = a
								continue
							}
						default:
							return nil, ErrOrAndMustBeArray
						}
					}

					// op is not $and or $or
					continue
				}

				nk := key
				if prefix != "" {
					nk = prefix + "." + key
				}

				stack = append(stack, stackFrame{nk, value})
			}

			if len(n) == 0 {
				result[prefix] = M{}
			}

		case []interface{}:
			tempResult := M{}
			for i := range n {
				switch t := n[i].(type) {
				case M:
					var newPrefix string
					if len(prefix) > 0 {
						newPrefix = fmt.Sprintf("%v.%v", prefix, i)
					} else {
						newPrefix = fmt.Sprintf("%v", i)
					}
					stack = append(stack, stackFrame{newPrefix, t})
				default:
					continue
				}
			}

			for k, v := range tempResult {
				result[k] = v
			}
		case nil:
			result[prefix] = n
		case string:
			if prefix != "" {
				result[prefix] = n
			}
		default:
			if prefix != "" {
				result[prefix] = n
			} else {
				if m, ok := n.(M); ok {
					if len(m) == 0 {
						result[prefix] = m
					}

					for k, v := range m {
						result[k] = v
					}
				}
			}
		}
	}

	return result, nil
}
