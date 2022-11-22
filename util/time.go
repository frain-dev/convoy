package util

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func StartOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func EndOfMonth(t time.Time) time.Time {
	return StartOfMonth(t).AddDate(0, 1, 0).Add(-time.Second)
}

func NewDateTime() *primitive.DateTime {
	d := primitive.NewDateTimeFromTime(time.Now())
	return &d
}
