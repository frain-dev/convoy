package v20240101

import (
	"time"
)

func transformDurationStringToInt(d string) (int64, error) {
	id, err := time.ParseDuration(d)
	if err != nil {
		return 0, err
	}

	return int64(id.Seconds()), nil
}

func transformIntToDurationString(t int64) (string, error) {
	td := time.Duration(t) * time.Second
	return td.String(), nil
}
