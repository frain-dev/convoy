package util

import (
	"errors"
	"fmt"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/frain-dev/convoy/config/algo"
	"github.com/frain-dev/convoy/datastore"
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

	govalidator.TagMap["supported_source"] = govalidator.Validator(func(source string) bool {
		sources := map[string]bool{
			string(datastore.HTTPSource):     true,
			string(datastore.RestApiSource):  true,
			string(datastore.PubSubSource):   true,
			string(datastore.DBChangeStream): true,
		}

		if _, ok := sources[source]; !ok {
			return false
		}

		return true
	})

	govalidator.TagMap["supported_verifier"] = govalidator.Validator(func(verifier string) bool {
		verifiers := map[string]bool{
			string(datastore.HMacVerifier):      true,
			string(datastore.BasicAuthVerifier): true,
			string(datastore.APIKeyVerifier):    true,
		}

		if _, ok := verifiers[verifier]; !ok {
			return false
		}

		return true
	})

	govalidator.TagMap["supported_encoding"] = govalidator.Validator(func(encoder string) bool {
		encoders := map[string]bool{
			string(datastore.Base64Encoding): true,
			string(datastore.HexEncoding):    true,
		}

		if _, ok := encoders[encoder]; !ok {
			return false
		}

		return true
	})
}
