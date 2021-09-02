package util

import "encoding/json"

func IsJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func JsonReMarshalString(s string) (string, error) {
	var i interface{}
	err := json.Unmarshal([]byte(s), &i)
	if err != nil {
		return "{}", err
	}
	output, err := json.Marshal(i)
	if err != nil {
		return "{}", err
	}
	return string(output), nil
}
