package compare

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

type CompareFunc func(x, y interface{}) bool

func defaultCompareMap() map[string]CompareFunc {
	return map[string]CompareFunc{
		"$gte":   gte,
		"$gt":    gt,
		"$lte":   lte,
		"$lt":    lt,
		"$in":    in,
		"$nin":   nin,
		"$eq":    eq,
		"$neq":   neq,
		"$or":    or,
		"$and":   and,
		"$exist": exist,
	}
}

func Compare(payload map[string]interface{}, filter map[string]interface{}) bool {
	return compare(payload, filter)
}

func compare(payload map[string]interface{}, filter map[string]interface{}) bool {
	pass := true
	cmp := defaultCompareMap()
	for key, filterVal := range filter {

		if strings.Contains(key, "$.") {
			possibleKeys, err := genCombos(key)
			if err != nil {
				fmt.Println(err.Error())
				return false
			}

			for _, newKey := range possibleKeys {
				if kk, ok := payload[newKey]; ok {
					check := cmp["$eq"](kk, filterVal)
					pass = pass || check
				}
			}
		}

		payloadVal, ok := payload[key]
		if !ok {
			if key == "$or" || key == "$and" {
				check := cmp[key](payload, filterVal)
				pass = pass && check
			}

			continue
		}

		switch v := filterVal.(type) {
		case map[string]interface{}:
			for vk, vv := range v {
				if vk == "$exist" {
					tmpFilter := map[string]interface{}{key: vv}
					check := cmp["$exist"](payload, tmpFilter)
					pass = pass && check

					continue
				}

				check := cmp[vk](payloadVal, vv)
				pass = pass && check
			}

		default:
			switch payloadVal.(type) {
			case []interface{}:
				check := cmp["$in"](payloadVal, filterVal)
				pass = pass && check

			case interface{}:
				check := cmp["$eq"](payloadVal, filterVal)
				pass = pass && check

			default:
				return pass
			}
		}
	}

	return pass
}

func gte(payload, filter interface{}) bool {
	p, ok := toFloat64(payload)
	if !ok {
		fmt.Printf("payload %v is not a valid number\n", payload)
		return false
	}

	f, ok := toFloat64(filter)
	if !ok {
		fmt.Printf("filter %v is not a valid number\n", filter)
		return false
	}

	return p >= f
}

func gt(payload, filter interface{}) bool {
	p, ok := toFloat64(payload)
	if !ok {
		fmt.Printf("payload %v is not a valid number\n", payload)
		return false
	}

	f, ok := toFloat64(filter)
	if !ok {
		fmt.Printf("filter %v is not a valid number\n", filter)
		return false
	}

	return p > f
}

func lte(payload, filter interface{}) bool {
	return !gt(payload, filter)
}

func lt(payload, filter interface{}) bool {
	return !gte(payload, filter)
}

// in finds if the filter in the payload.
// filter could be a string, number or bool, payload is an array
// there are two scenarios
//  1. when we query directly on an array field
//  2. when we try to check if a value is in a given array
func in(payload, filter interface{}) bool {
	p, ok := filter.([]interface{})
	// scenario 2. We used the $in op
	if ok {
		filter = payload
	} else {
		// scenario 1. We are querying on an array field
		p, ok = payload.([]interface{})
		if !ok {
			fmt.Printf("%+v is not a valid slice\n", payload)
			return false
		}
	}

	sort.SliceStable(p, func(i, j int) bool {
		switch pi := p[i].(type) {
		case string:
			return pi < p[j].(string)
		case float64:
			return pi < p[j].(float64)
		case int:
			return pi < p[j].(int)
		}

		return false
	})

	index := sort.Search(len(p), func(i int) bool {
		return reflect.DeepEqual(p[i], filter)
	})

	return index < len(p)
}

func nin(payload, filter interface{}) bool {
	return !in(payload, filter)
}

// eq checks whether x, y are deeply eq
func eq(x, y interface{}) bool {
	// if the x value is numeric (int/int8-int64/float32/float64) then convert to float64
	if fx, ok := toFloat64(x); ok {
		x = fx
	}

	// if the y value is numeric (int/int8-int64/float32/float64) then convert to float64
	if fy, ok := toFloat64(y); ok {
		y = fy
	}

	fmt.Printf("x: %v y: %v\n", x, y)

	return reflect.DeepEqual(x, y)
}

// eq checks whether x, y are deeply eq
func neq(x, y interface{}) bool {
	return !eq(x, y)
}

// or evaluate matches across an array of conditions. The array of conditions can contain any other valid json schema.
func or(payload, filter interface{}) bool {
	check := false
	f, ok := filter.([]interface{})
	if !ok {
		fmt.Printf("filter %v is not valid json\n", filter)
		return false
	}

	p, ok := payload.(map[string]interface{})
	if !ok {
		fmt.Printf("payload %v is not valid json\n", payload)
		return false
	}

	for _, value := range f {
		check = check || compare(p, value.(map[string]interface{}))
	}

	return check
}

// and evaluate matches across an array of conditions. The array of conditions can contain any other valid json schema.
func and(payload, filter interface{}) bool {
	check := true
	f, ok := filter.([]interface{})
	if !ok {
		fmt.Printf("filter %v is not valid json\n", filter)
		return false
	}

	p, ok := payload.(map[string]interface{})
	if !ok {
		fmt.Printf("payload %v is not valid json\n", payload)
		return false
	}

	for _, value := range f {
		check = check && compare(p, value.(map[string]interface{}))
	}

	return check
}

// exist requires a field to be undefined when false and array, number, object, string, boolean or null when true.
func exist(payload, filter interface{}) bool {
	f, ok := filter.(map[string]interface{})
	if !ok {
		fmt.Printf("filter %v is not valid json\n", filter)
		return false
	}

	p, ok := payload.(map[string]interface{})
	if !ok {
		fmt.Printf("payload %v is not valid json\n", payload)
		return false
	}

	var want bool
	var key string

	for k, v := range f {
		key = k
		want = v.(bool)
	}

	b := false
	for k := range p {
		if _, ok := f[k]; ok {
			if k == key {
				b = true
				break
			}
		}
	}

	return b == want
}

// toFloat64 converts interface{} value to float64 if value is numeric else return false
func toFloat64(v interface{}) (float64, bool) {
	var f float64
	flag := true
	// as Go convert the json Numeric value to float64
	switch u := v.(type) {
	case int:
		f = float64(u)
	case int8:
		f = float64(u)
	case int16:
		f = float64(u)
	case int32:
		f = float64(u)
	case int64:
		f = float64(u)
	case float32:
		f = float64(u)
	case float64:
		f = u
	default:
		flag = false
	}
	return f, flag
}

// genCombos takes an input string s and a maximum integer value n,
// replaces all occurrences of "$" with integers from 0 to n,
// and returns a slice of strings representing all possible combinations.
// If the number of segments in the input string is more than 3, the function
// returns an error with a message indicating the number of segments.
func genCombos(s string) ([]string, error) {
	segments := strings.Split(s, "$")
	n := len(segments) - 1
	if n > 3 {
		return nil, fmt.Errorf("too many segments, expected at most 3 but got %d", n)
	}

	combinations := make([]string, 2*len(segments)-1)
	for i := range combinations {
		if i%2 == 0 {
			combinations[i] = segments[i/2]
		} else {
			combinations[i] = "$"
		}
	}
	return generateCombinations(combinations, 1, n), nil
}

// generateCombinations takes an array of strings representing a combination of
// non-replaced segments and "$" characters, a current index, and a maximum integer value n,
// generates all possible combinations of integers from 0 to n for each "$" character,
// and returns a slice of strings representing all possible combinations.
func generateCombinations(combinations []string, index int, n int) []string {
	if index >= len(combinations) {
		return []string{strings.Join(combinations, "")}
	}
	if combinations[index] == "$" {
		result := make([]string, 0)
		for i := 0; i <= n; i++ {
			combinations[index] = strconv.Itoa(i)
			newCombinations := generateCombinations(combinations, index+2, n)
			result = append(result, newCombinations...)
		}
		combinations[index] = "$"
		return result
	} else {
		newCombinations := generateCombinations(combinations, index+2, n)
		return newCombinations
	}
}
