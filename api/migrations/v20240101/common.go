package v20240101

import (
	"fmt"
	"time"

	"github.com/fatih/structs"
)

type direction string

const (
	forward  direction = "forward"
	backward direction = "backward"
)

func transformDurationStringToInt(d string) (uint64, error) {
	id, err := time.ParseDuration(d)
	if err != nil {
		return 0, err
	}

	return uint64(id.Seconds()), nil
}

func transformIntToDurationString(t uint64) (string, error) {
	td := time.Duration(t) * time.Second
	return td.String(), nil
}

func migrateEndpoint(oldPayload, newPayload interface{}, direction direction) error {
	oldStruct := structs.New(oldPayload)
	newStruct := structs.New(newPayload)

	var err error
	for _, f := range oldStruct.Fields() {
		if f.IsZero() {
			continue
		}

		value := f.Value()
		jsonTag := f.Tag("json")
		if jsonTag == "http_timeout" || jsonTag == "rate_limit_duration" {
			switch direction {
			case forward:
				newValue, ok := f.Value().(string)
				if !ok {
					return fmt.Errorf("invalid type for %s field", jsonTag)
				}

				value, err = transformDurationStringToInt(newValue)
				if err != nil {
					return err
				}
			case backward:
				newValue, ok := f.Value().(uint64)
				if !ok {
					return fmt.Errorf("invalid type for %s field", jsonTag)
				}

				value, err = transformIntToDurationString(newValue)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("invalid direction %s", direction)
			}
		}

		newStruct.Field(f.Name()).Set(value)
	}

	return nil
}
