package util

import (
	"errors"
	"fmt"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/frain-dev/convoy/config/algo"
)

func Validate(dst interface{}) error {
	_, err := govalidator.ValidateStruct(dst)

	var messages []string

	if err != nil {
		errs := govalidator.ErrorsByField(err)
		for field, message := range errs {
			messages = append(messages, fmt.Sprintf("%s:%s", field, message))
		}

		return errors.New(strings.Join(messages, ", "))
	}

	return nil
}

func init() {
	govalidator.TagMap["supported_hash"] = govalidator.Validator(func(hash string) bool {
		if _, ok := algo.M[hash]; !ok {
			return false
		}

		return true
	})
}
