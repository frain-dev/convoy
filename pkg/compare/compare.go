package compare

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

var ErrTrailingDollarOpNotAllowed = errors.New("invalid filter syntax, found trailing $")

type CompareFunc func(x, y interface{}) (bool, error)

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

func Compare(payload map[string]interface{}, filter map[string]interface{}) (bool, error) {
	return compare(payload, filter)
}

func compare(payload map[string]interface{}, filter map[string]interface{}) (bool, error) {
	var pass []bool
	cmp := defaultCompareMap()
	for key, filterVal := range filter {
		slen := len(key)

		// the filter ends with a trailing .$
		if key[slen-1] == '$' && key[slen-2] == '.' {
			return false, ErrTrailingDollarOpNotAllowed
		}

		if strings.Contains(key, "$.") {
			var chks []bool
			possibleKeys, err := genCombos(key)
			if err != nil {
				return false, err
			}

			for _, newKey := range possibleKeys {
				if _, ok := payload[newKey]; ok {
					chk, err := compare(payload, map[string]interface{}{newKey: filterVal})
					if err != nil {
						return false, err
					}
					chks = append(chks, chk)
				}
			}

			var chkReduced bool

			if len(chks) > 0 {
				chkReduced = chks[0]
			}

			for i := range chks {
				if i == 0 {
					continue
				}
				chkReduced = chkReduced || chks[i]
			}

			pass = append(pass, chkReduced)
		}

		payloadVal, ok := payload[key]
		if !ok {
			if key == "$or" || key == "$and" {
				check, err := cmp[key](payload, filterVal)
				if err != nil {
					return false, err
				}
				pass = append(pass, check)
			}

			continue
		}

		switch v := filterVal.(type) {
		case map[string]interface{}:
			for vk, vv := range v {
				if vk == "$exist" {
					tmpFilter := map[string]interface{}{key: vv}
					check, err := cmp["$exist"](payload, tmpFilter)
					if err != nil {
						return false, err
					}
					pass = append(pass, check)

					continue
				}

				check, err := cmp[vk](payloadVal, vv)
				if err != nil {
					return false, err
				}
				pass = append(pass, check)
			}

		default:
			switch payloadVal.(type) {
			case []interface{}:
				check, err := cmp["$in"](payloadVal, filterVal)
				if err != nil {
					return false, err
				}
				pass = append(pass, check)

			case interface{}:
				check, err := cmp["$eq"](payloadVal, filterVal)
				if err != nil {
					return false, err
				}
				pass = append(pass, check)
			}

		}
	}

	passReduced := false
	if len(filter) == 0 {
		passReduced = true
	}

	if len(pass) > 0 {
		passReduced = pass[0]
	}

	for i := range pass {
		if i == 0 {
			continue
		}
		passReduced = passReduced && pass[i]
	}

	return passReduced, nil
}

func gte(payload, filter interface{}) (bool, error) {
	p, ok := toFloat64(payload)
	if !ok {
		fmt.Printf("payload %v is not a valid number\n", payload)
		return false, nil
	}

	f, ok := toFloat64(filter)
	if !ok {
		fmt.Printf("filter %v is not a valid number\n", filter)
		return false, nil
	}

	return p >= f, nil
}

func gt(payload, filter interface{}) (bool, error) {
	p, ok := toFloat64(payload)
	if !ok {
		fmt.Printf("payload %v is not a valid number\n", payload)
		return false, fmt.Errorf("payload %v is not a valid number\n", payload)
	}

	f, ok := toFloat64(filter)
	if !ok {
		fmt.Printf("filter %v is not a valid number\n", filter)
		return false, fmt.Errorf("filter %v is not a valid number\n", filter)
	}

	return p > f, nil
}

func lte(payload, filter interface{}) (bool, error) {
	chk, err := gt(payload, filter)
	return !chk, err
}

func lt(payload, filter interface{}) (bool, error) {
	chk, err := gte(payload, filter)
	return !chk, err
}

// in finds if the filter in the payload.
// filter could be a string, number or bool, payload is an array
// there are two scenarios
//  1. when we query directly on an array field
//  2. when we try to check if a value is in a given array
func in(payload, filter interface{}) (bool, error) {
	p, ok := filter.([]interface{})
	// scenario 2. We used the $in op
	if ok {
		filter = payload
	} else {
		// scenario 1. We are querying on an array field
		p, ok = payload.([]interface{})
		if !ok {
			fmt.Printf("%+v is not a valid slice\n", payload)
			return false, fmt.Errorf("%+v is not a valid slice\n", payload)
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

	found := false
	for _, v := range p {
		if v == filter {
			found = true
		}
	}

	return found, nil
}

func nin(payload, filter interface{}) (bool, error) {
	chk, err := in(payload, filter)
	return !chk, err
}

// eq checks whether x, y are deeply eq
func eq(x, y interface{}) (bool, error) {
	// if the x value is numeric (int/int8-int64/float32/float64) then convert to float64
	if fx, ok := toFloat64(x); ok {
		x = fx
	}

	// if the y value is numeric (int/int8-int64/float32/float64) then convert to float64
	if fy, ok := toFloat64(y); ok {
		y = fy
	}

	return reflect.DeepEqual(x, y), nil
}

// eq checks whether x, y are deeply eq
func neq(x, y interface{}) (bool, error) {
	chk, err := eq(x, y)
	return !chk, err
}

// or evaluate matches across an array of conditions. The array of conditions can contain any other valid json schema.
func or(payload, filter interface{}) (bool, error) {
	check := false
	f, ok := filter.([]interface{})
	if !ok {
		fmt.Printf("filter %v is not valid json\n", filter)
		return false, fmt.Errorf("filter %v is not valid json\n", filter)
	}

	p, ok := payload.(map[string]interface{})
	if !ok {
		fmt.Printf("payload %v is not valid json\n", payload)
		return false, fmt.Errorf("payload %v is not valid json\n", payload)
	}

	for _, value := range f {
		chk, err := compare(p, value.(map[string]interface{}))
		if err != nil {
			return false, err
		}
		check = check || chk
	}

	return check, nil
}

// and evaluate matches across an array of conditions. The array of conditions can contain any other valid json schema.
func and(payload, filter interface{}) (bool, error) {
	check := true

	f, ok := filter.([]interface{})
	if !ok {
		fmt.Printf("filter %v is not valid json\n", filter)
		return false, fmt.Errorf("filter %v is not valid json\n", filter)
	}

	p, ok := payload.(map[string]interface{})
	if !ok {
		fmt.Printf("payload %v is not valid json\n", payload)
		return false, fmt.Errorf("payload %v is not valid json\n", payload)
	}

	for _, value := range f {
		chk, err := compare(p, value.(map[string]interface{}))
		if err != nil {
			return false, err
		}

		check = check && chk
	}

	return check, nil
}

// exist requires a field to be undefined when false and array, number, object, string, boolean or null when true.
func exist(payload, filter interface{}) (bool, error) {
	f, ok := filter.(map[string]interface{})
	if !ok {
		fmt.Printf("filter %v is not valid json\n", filter)
		return false, fmt.Errorf("filter %v is not valid json\n", filter)
	}

	p, ok := payload.(map[string]interface{})
	if !ok {
		fmt.Printf("payload %v is not valid json\n", payload)
		return false, fmt.Errorf("payload %v is not valid json\n", payload)
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

	return b == want, nil
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
	n := 0
	for i := 0; i < len(s); {
		if s[i] == '$' {
			if i < len(s)-1 && s[i+1] == '.' {
				n++
				i += 2
				continue
			}
		}

		i++
	}

	if n > 3 {
		return nil, fmt.Errorf("too many segments, expected at most 3 but got %d", n)
	}

	segments := strings.Split(s, "$")
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
