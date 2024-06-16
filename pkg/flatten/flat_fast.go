package flatten

import (
	"fmt"
	"strings"
)

type stackFrame struct {
	prefix string
	nested interface{}
	// setPrevious *interface{}
}

//func (s stackFrame) setPrev(v interface{}) {
//	rv := reflect.ValueOf(s.setPrevious)
//	rv.Set(reflect.ValueOf(v))
//}

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

func flatFast2(prefix string, nested interface{}) (M, error) {
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
				if strings.HasPrefix(key, "$") && !strings.HasPrefix(key, "$.") {
					if !isKeyValidOperator(key) {
						return nil, fmt.Errorf("%s starts with a $ and is not a valid operator", key)
					}

					if key == "$or" || key == "$and" {
					}

					// op is not $and or $or
					continue
				}

				nk := key
				if len(prefix) > 0 {
					nk = prefix + "." + key
				}

				stack = append(stack, stackFrame{nk, value})
			}

			// nothing in n, but it's prefix existed, so add empty map to result
			if len(n) == 0 {
				result[prefix] = M{}
			}
		case []interface{}:
			// either this is a nested array of maps, or a string or int float array
			// if it is the latter, we don't need to expand it, just add it to the result
			if isHomogenousArray(n) {
				result[prefix] = n
				continue
			}

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
					var newPrefix string
					if len(prefix) > 0 {
						newPrefix = fmt.Sprintf("%v.%v", prefix, i)
					} else {
						newPrefix = fmt.Sprintf("%v", i)
					}
					result[newPrefix] = t
					continue
				}
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

func isHomogenousArray(v []interface{}) bool {
	if len(v) == 0 {
		return true
	}

	// arrays in json are homogenous, so if the first element of this array is int or float
	// the remaining are the same type.
	switch v[0].(type) {
	case int, float64, string:
		return true
	}

	return false
}
