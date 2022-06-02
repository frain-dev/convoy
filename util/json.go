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
	var syntaxError *json.SyntaxError
	var unmarshalTypeError *json.UnmarshalTypeError

	err := json.NewDecoder(r.Body).Decode(dst)
	if err != nil {
		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return ErrEmptyBody

		default:
			return err
		}

	}

	return nil
}
