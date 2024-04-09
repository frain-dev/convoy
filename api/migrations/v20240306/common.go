package v20240306

import (
	"github.com/fatih/structs"
)

func migrateEndpoint(oldPayload, newPayload interface{}) error {
	oldStruct := structs.New(oldPayload)
	newStruct := structs.New(newPayload)

	var err error
	for _, f := range oldStruct.Fields() {
		if f.IsZero() {
			continue
		}

		value := f.Value()
		jsonTag := f.Tag("json")

		switch jsonTag {
		case "url":
			err = newStruct.Field("TargetURL").Set(value)
			if err != nil {
				return err
			}
		case "name":
			err = newStruct.Field("Title").Set(value)
			if err != nil {
				return err
			}
		default:
			err = newStruct.Field(f.Name()).Set(value)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
