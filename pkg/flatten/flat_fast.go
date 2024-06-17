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

func flatFast2(prefix string, nested interface{}) (M, error) {
	stack := []stackFrame{{prefix, nested}}
	result := M{}

	if nested == nil {
		return M{}, nil
	}

	switch v := nested.(type) {
	case M:
		if len(v) == 0 {
			return M{}, nil
		}
	case []interface{}:
		if len(v) == 0 {
			return M{}, nil
		}
	}

	// outer:
	for len(stack) > 0 {
		// Pop from stack
		currentFrame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		prefix := currentFrame.prefix
		nested := currentFrame.nested

		switch n := nested.(type) {
		case M:
			if len(n) == 0 {
				// nothing in n, but its prefix exists, so add empty map to result
				result[prefix] = M{}
				continue
			}

			for key, value := range n {
				if strings.HasPrefix(key, "$") && !strings.HasPrefix(key, "$.") {
					if !isKeyValidOperator(key) {
						return nil, fmt.Errorf("%s starts with a $ and is not a valid operator", key)
					}

					if key == "$or" || key == "$and" {
						switch a := value.(type) {
						case []interface{}:
							for i := range a {
								newM, err := flatFast2("", a[i])
								if err != nil {
									return nil, err
								}
								fmt.Println("nnn", newM)
								a[i] = newM
							}

							k := M{key: a}
							if len(prefix) > 0 {
								result[prefix] = k
							} else {
								result = k
							}

						default:
							return nil, ErrOrAndMustBeArray
						}
					}
					//
					k := M{key: value}
					if len(prefix) > 0 {
						result[prefix] = k
					} else {
						result = k
					}

					continue
				}

				nk := key
				if len(prefix) > 0 {
					nk = prefix + "." + key
				}

				stack = append(stack, stackFrame{nk, value})
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
		case string, int:
			if prefix != "" {
				result[prefix] = n
			}
		default:
			if prefix != "" {
				result[prefix] = n
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
