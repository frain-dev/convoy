package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

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

var ErrEmptyBody = errors.New("body must not be empty")

func ReadJSON(r *http.Request, dst interface{}) error {
	err := json.NewDecoder(r.Body).Decode(dst)
	if err != nil {
		if syntaxError, ok := errors.AsType[*json.SyntaxError](err); ok {
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return errors.New("body contains badly-formed JSON")
		}
		if unmarshalTypeError, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		}
		if errors.Is(err, io.EOF) {
			return ErrEmptyBody
		}
		return err
	}

	return nil
}
