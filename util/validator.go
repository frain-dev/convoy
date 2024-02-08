package util

import (
	"errors"
	"fmt"
	"strings"
	"time"

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
		return datastore.SourceType(source).IsValid()
	})

	govalidator.TagMap["supported_verifier"] = govalidator.Validator(func(verifier string) bool {
		verifiers := map[string]bool{
			string(datastore.NoopVerifier):      true,
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

	govalidator.TagMap["supported_retry_strategy"] = govalidator.Validator(func(encoder string) bool {
		encoders := map[string]bool{
			string(datastore.LinearStrategyProvider):      true,
			string(datastore.ExponentialStrategyProvider): true,
		}

		if _, ok := encoders[encoder]; !ok {
			return false
		}

		return true
	})

	govalidator.TagMap["supported_storage"] = govalidator.Validator(func(encoder string) bool {
		encoders := map[string]bool{
			string(datastore.S3):     true,
			string(datastore.OnPrem): true,
		}

		if _, ok := encoders[encoder]; !ok {
			return false
		}

		return true
	})

	govalidator.TagMap["duration"] = govalidator.Validator(func(duration string) bool {
		_, err := time.ParseDuration(duration)

		return err == nil
	})

	govalidator.TagMap["supported_pub_sub"] = govalidator.Validator(func(pubsub string) bool {
		pubsubs := map[string]bool{
			string(datastore.SqsPubSub):    true,
			string(datastore.GooglePubSub): true,
			string(datastore.KafkaPubSub):  true,
			string(datastore.AmqpPubSub):   true,
		}

		if _, ok := pubsubs[pubsub]; !ok {
			return false
		}

		return true
	})
}
