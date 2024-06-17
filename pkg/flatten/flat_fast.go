package flatten

import (
	"fmt"
	"strings"
)

type stackFrame struct {
	prefix   string
	nested   interface{}
	prevKeys []interface{}
}

func (s *stackFrame) hasPrevKeys() bool {
	return len(s.prevKeys) > 0
}

func flatFast2(prefix string, nested interface{}) (M, error) {
	stack := []stackFrame{{nested: nested, prefix: prefix}}
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
			if len(n) == 0 {
				// nothing in n, but it's prefix exists, so add empty map to result
				result[prefix] = M{}
				continue
			}

			for key, value := range n {
				if strings.HasPrefix(key, "$") && !strings.HasPrefix(key, "$.") {
					if !isKeyValidOperator(key) {
						return nil, fmt.Errorf("%s starts with a $ and is not a valid operator", key)
					}

					if key == "$or" || key == "$and" {
						switch value.(type) {
						case []interface{}:
							// do nothing, we just need to make sure $or and $and hava array values
						default:
							return nil, ErrOrAndMustBeArray
						}
					}

					switch value.(type) {
					case M:
						sf := stackFrame{
							prefix:   "", // empty prefix cause we've encountered an op
							nested:   value,
							prevKeys: append(currentFrame.prevKeys, prefix, key),
						}

						stack = append(stack, sf)
					case []interface{}:
						sf := stackFrame{
							prefix: "", // empty prefix cause we've encountered an op
							nested: value,
						}

						if len(prefix) > 0 {
							sf.prevKeys = append(currentFrame.prevKeys, prefix, key)
						} else {
							sf.prevKeys = append(currentFrame.prevKeys, key)
						}

						stack = append(stack, sf)
					default:
						if currentFrame.hasPrevKeys() {
							if len(prefix) > 0 {
								setNestedKeys(&result, append(currentFrame.prevKeys, key), value)
							} else {
								setNestedKeys(&result, currentFrame.prevKeys, value)
							}
						} else {
							result[prefix] = value
						}
					}

					continue
				}

				nk := key
				if len(prefix) > 0 {
					nk = prefix + "." + key
				}

				sf := stackFrame{
					prefix: nk,
					nested: value,
				}

				if currentFrame.hasPrevKeys() {
					if len(prefix) > 0 {
						sf.prevKeys = append(currentFrame.prevKeys, prefix, key)
					} else {
						sf.prevKeys = append(currentFrame.prevKeys, key)
					}
				}

				stack = append(stack, sf)
			}
		case []interface{}:
			// either this is a nested array of maps, or a string or int float array
			// if it is the latter, we don't need to expand it, just add it to the result
			if isHomogenousArray(n) {
				if currentFrame.hasPrevKeys() {
					setNestedKeys(&result, currentFrame.prevKeys, n)
				} else {
					result[prefix] = n
				}
				continue
			}

			for i := range n {
				switch n[i].(type) {
				case M:
					var newPrefix interface{}
					if len(prefix) > 0 {
						newPrefix = fmt.Sprintf("%v.%v", prefix, i)
					} else {
						newPrefix = i
					}

					if currentFrame.hasPrevKeys() {
						stack = append(stack, stackFrame{
							nested:   n[i],
							prevKeys: append(currentFrame.prevKeys, newPrefix),
						})
					} else {
						stack = append(stack, stackFrame{nested: n[i], prefix: newPrefix.(string)})
					}
				default:
					// if the array is not homogenous then all it's elements must be of type M.
					continue
				}
			}
		case string, nil:
			if len(prefix) > 0 {
				result[prefix] = n
			}
		default:
			if prefix != "" {
				fmt.Println("IN DEFAULT 111")
				result[prefix] = n
			} else {
				fmt.Println("IN DEFAULT 222")
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

//// setNestedKeys follows the nested prevKeys, and sets the last key to v
//func setNestedKeys(result M, prevKeys []interface{}, v interface{}) {
//	if len(prevKeys) == 0 {
//		return
//	}
//
//	mapper := result
//	var (
//		key    string
//		newMap M
//	)
//	for i := 0; i < len(prevKeys)-1; i++ {
//		// Ensure the key exists and is a map, otherwise create a new map
//		if nextMapper, ok := mapper[key].(M); ok {
//			mapper = nextMapper
//		} else {
//			newMap = M{}
//			mapper = newMap
//			mapper[key] = newMap
//		}
//	}
//
//	// Set the final key to the value
//	finalKey := prevKeys[len(prevKeys)-1]
//	mapper[finalKey] = v
//}
//

// setNestedKeys follows the nested prevKeys, and sets the last key to v
func setNestedKeys(result *M, prevKeys []interface{}, v interface{}) {
	if len(prevKeys) == 0 {
		return
	}

	var (
		mapper interface{} = *result
		newMap interface{}
		newArr []interface{}
	)

	for i := 0; i < len(prevKeys)-1; i++ {
		switch key := prevKeys[i].(type) {
		case string:

			oldMapper := mapper.(M)

			if nextMapper, ok := mapper.(M)[key]; ok {
				mapper = nextMapper
			} else {
				newMap = M{}
				mapper.(M)[key] = newMap
				mapper = newMap
			}

			if len(prevKeys) > i+1 {
				if index, ok := prevKeys[i+1].(int); ok {
					arr, ok := oldMapper[key].([]interface{})
					if !ok {
						arr = make([]interface{}, index+1)
						oldMapper[key] = arr
					} else if len(arr) <= index {
						newArr = make([]interface{}, index+1)
						copy(newArr, arr)
						arr = newArr
						oldMapper[key] = arr
					}

					if arr[index] == nil {
						newMap = M{}
						arr[index] = newMap
						oldMapper[key] = newMap
					} else {
						oldMapper[key] = arr[index]
					}
				}

				mapper = oldMapper
			}

		case int:
			continue
		default:
			return // Invalid key type
		}
	}

	// Set the final key to the value
	finalKey := prevKeys[len(prevKeys)-1]
	switch key := finalKey.(type) {
	case string:
		mapper.(M)[key] = v
	case int:
		arr, ok := mapper.([]interface{})
		if !ok {
			arr = make([]interface{}, key+1)
			mapper = arr
		} else if len(arr) <= key {
			newArr = make([]interface{}, key+1)
			copy(newArr, arr)
			arr = newArr
			mapper = newArr
		}
		arr[key] = v
	default:
		return // Invalid key type
	}
}

// setNestedKeys follows the nested prevKeys, and sets the last key to v
//func setNestedKeys(result *M, prevKeys []interface{}, v interface{}) {
//    if len(prevKeys) == 0 {
//        return
//    }
//
//    var (
//        mapper interface{} = result
//        newMap M
//        newArr []interface{}
//    )
//
//    for i := 0; i < len(prevKeys)-1; i++ {
//        switch key := prevKeys[i].(type) {
//        case string:
//            if nextMapper, ok := (*(mapper.(*M)))[key]; ok {
//                mapper = &nextMapper
//            } else {
//                newMap = M{}
//                (*(mapper.(*M)))[key] = &newMap
//                mapper = &newMap
//            }
//        case int:
//            // if current key is an index, then the last key must be an array key
//            nm := mapper.(*M)
//
//            pk, ok := prevKeys[i-1].(string)
//            if !ok {
//                // malformed data, kindly piss off
//                return
//            }
//
//            prevM := (*nm)[pk]
//
//            arr, ok := prevM.([]interface{})
//            if !ok {
//                arr = make([]interface{}, key+1)
//                (*nm)[pk] = arr
//            } else if len(arr) <= key {
//                newArr = make([]interface{}, key+1)
//                copy(newArr, arr)
//                arr = newArr
//                (*nm)[pk] = newArr
//            }
//
//            if arr[key] == nil {
//                newMap = M{}
//                arr[key] = &newMap
//                mapper = &newMap
//            } else {
//                mapper = &arr[key]
//            }
//        default:
//            return // Invalid key type
//        }
//    }
//
//    // Set the final key to the value
//    finalKey := prevKeys[len(prevKeys)-1]
//    switch key := finalKey.(type) {
//    case string:
//        (*(mapper.(*M)))[key] = v
//    case int:
//        arr, ok := mapper.(*[]interface{})
//        if !ok {
//            *arr = make([]interface{}, key+1)
//            mapper = arr
//        } else if len(*arr) <= key {
//            newArr = make([]interface{}, key+1)
//            copy(newArr, *arr)
//            *arr = newArr
//            mapper = &newArr
//        }
//        (*arr)[key] = v
//    default:
//        return // Invalid key type
//    }
//}

func peekMap(mm M, nextKey string) {
}
