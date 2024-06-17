package flatten

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// This package https://github.com/tidwall/gjson might
// offer us a good technique for significantly reducing the number of allocations we do here

var operators = map[string]struct{}{
	"$gte":   {},
	"$gt":    {},
	"$lte":   {},
	"$lt":    {},
	"$in":    {},
	"$nin":   {},
	"$eq":    {},
	"$neq":   {},
	"$or":    {},
	"$and":   {},
	"$exist": {},
	"$regex": {},
}

type stackFrame struct {
	prefix string
	nested interface{}
}

var ErrOrAndMustBeArray = errors.New("the value of $or and $and must be an array")

// Flatten flattens extended JSON which is used to build and store queries.
//
// Payloads that look like this
//
//	{
//		"person": {
//			"age": {
//				"$gte": 5
//			}
//		}
//	}
//
// would become
//
//	{
//		"person.age": {
//		  "$gte": 5
//		}
//	}
func Flatten(input interface{}) (M, error) {
	return flatten("", input)
}

func FlattenWithPrefix(prefix string, input interface{}) (M, error) {
	return flatten(prefix, input)
}

func flatten(prefix string, nested interface{}) (M, error) {
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

	b := &strings.Builder{}

	var (
		// reused vars
		currentFrame stackFrame
		prefixInner  string
		nestedInner  interface{}

		ok        bool
		si        string
		newPrefix string
	)

	// outer:
	for len(stack) > 0 {
		// Pop from stack
		currentFrame = stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		prefixInner = currentFrame.prefix
		nestedInner = currentFrame.nested

		switch n := nestedInner.(type) {
		case M:
			if len(n) == 0 {
				// nothing in n, but its prefix exists, so add empty map to result
				result[prefixInner] = M{}
				continue
			}

			for key, value := range n {
				if strings.HasPrefix(key, "$") && !strings.HasPrefix(key, "$.") {
					if _, ok = operators[key]; !ok {
						return nil, fmt.Errorf("%s starts with a $ and is not a valid operator", key)
					}

					if key == "$or" || key == "$and" {
						switch a := value.(type) {
						case []interface{}:

							// a might look like:
							//{
							//    "person": M{
							//        "age": M{
							//            "$in": []int{10, 11, 12},
							//        },
							//    },
							//},

							// In the future, we can flatten large $and and $or array items concurrently
							// say if len(a) > 10, use goroutines to concurrently flatten each item

							for i := range a {
								// we only recurse for $or or $and operators
								// tried the stackFrame, but it was a much more complex solution, so going with this for now
								// flatten the current item in the array
								newM, err := flatten("", a[i])
								if err != nil {
									return nil, err
								}

								// change the item to the flattened version
								a[i] = newM
							}

							// by the time we get here a will look like:
							//{
							//    "person.age": M{
							//    "$in": []int{10, 11, 12},
							//},
							k := M{key: a} // set key [$or or $and] to the new value of a and set it in result
							if len(prefixInner) > 0 {
								result[prefixInner] = k
							} else {
								result = k
							}

						default:
							return nil, ErrOrAndMustBeArray
						}
					}

					// it's one of the unary ops [$in, $lt, ...] these do not require recursion or expansion
					// and so forth so just set it directly
					k := M{key: value}
					if len(prefixInner) > 0 {
						result[prefixInner] = k
					} else {
						result = k
					}

					continue
				}

				if len(prefixInner) > 0 {
					b.Grow(len(key) + len(prefixInner) + 1)
					b.WriteString(prefixInner)
					b.WriteString(".")
					b.WriteString(key)
					key = b.String()
					b.Reset()
				}

				stack = append(stack, stackFrame{key, value})
			}
		case []interface{}:
			// either this is a nested array of maps, or a string or int float array
			// if it is the latter, we don't need to expand it, just add it to the result
			if isHomogenousArray(n) {
				result[prefixInner] = n
				continue
			}

			for i := range n {
				switch t := n[i].(type) {
				case M:
					if len(prefixInner) > 0 {
						si = strconv.Itoa(i)

						b.Grow(len(si) + len(prefixInner) + 1)
						b.WriteString(prefixInner)
						b.WriteString(".")
						b.WriteString(si)

						newPrefix = b.String()

						b.Reset()
					} else {
						newPrefix = strconv.Itoa(i)
					}
					stack = append(stack, stackFrame{newPrefix, t})
				}
			}
		// default will handle string and int and nil
		default:
			if prefixInner != "" {
				result[prefixInner] = n
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
