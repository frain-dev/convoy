package flatten

import (
	"fmt"
	"strings"
)

func Flatten(input map[string]interface{}) (map[string]interface{}, error) {
	return flatten("", input)
}

func flatten(prefix string, nested interface{}) (map[string]interface{}, error) {
	f := map[string]interface{}{}

	switch n := nested.(type) {
	case map[string]interface{}:
		for key, value := range n {
			if strings.HasPrefix(key, "$") {
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
	default:
		f[prefix] = n
	}

	return f, nil
}
