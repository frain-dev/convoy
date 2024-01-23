package v20240101

import (
	"time"
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
