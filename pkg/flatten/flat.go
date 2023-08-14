package flatten

import (
	"errors"
	"fmt"
	"strings"
)

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
func Flatten(input interface{}) (map[string]interface{}, error) {
	return flatten("", input)
}

func FlattenWithPrefix(prefix string, input interface{}) (map[string]interface{}, error) {
	return flatten(prefix, input)
}

func flatten(prefix string, nested interface{}) (map[string]interface{}, error) {
	f := map[string]interface{}{}

	switch n := nested.(type) {
	case map[string]interface{}:
		for key, value := range n {
			if strings.HasPrefix(key, "$") && !strings.HasPrefix(key, "$.") {
				if !isKeyValidOperator(key) {
					return nil, fmt.Errorf("%s starts with a $ and is not a valid operator", key)
				}

				if key == "$or" || key == "$and" {
					switch a := value.(type) {
					case []interface{}:
						for i := range a {
							t, err := flatten("", a[i])
							if err != nil {
								return nil, err
							}

							a[i] = t
						}

						f[key] = a
						return f, nil
					case []map[string]interface{}:
						for i := range a {
							t, err := flatten("", a[i])
							if err != nil {
								return nil, err
							}

							a[i] = t
						}

						f[key] = a
						return f, nil
					default:
						return nil, ErrOrAndMustBeArray
					}
				}

				// op is not $and or $or
				continue
			}

			m, err := flatten(key, value)
			if err != nil {
				return nil, err
			}

			for mKey, mValue := range m {
				nKey := mKey
				if len(prefix) > 0 {
					nKey = fmt.Sprintf("%s.%s", prefix, mKey)
				}
				f[nKey] = mValue
			}

			// the map is empty so flatten the parent.child
			// and set the value to the new key
			if len(m) == 0 {
				if len(prefix) > 0 {
					key = fmt.Sprintf("%s.%s", prefix, key)
				}
				f[key] = value
			}
		}
	case []interface{}:
		ff := map[string]interface{}{}

		for i := range n {
			switch t := n[i].(type) {
			case map[string]interface{}:
				var p string
				if len(prefix) > 0 {
					p = fmt.Sprintf("%v.%v", prefix, i)
				} else {
					p = fmt.Sprintf("%v", i)
				}

				t, err := flatten(p, t)
				if err != nil {
					return nil, err
				}

				for k, v := range t {
					ff[k] = v
				}
			default:
				continue
			}
		}

		for k, v := range ff {
			f[k] = v
		}
	case nil:
	default:
		if prefix != "" {
			f[prefix] = n
		} else {
			f = n.(map[string]interface{})
		}
	}

	return f, nil
}

func isKeyValidOperator(op string) bool {
	operators := []string{
		"$gte",
		"$gt",
		"$lte",
		"$lt",
		"$in",
		"$nin",
		"$eq",
		"$neq",
		"$or",
		"$and",
		"$exist",
		"$regex",
	}

	for _, o := range operators {
		if o == op {
			return true
		}
	}

	return false
}
