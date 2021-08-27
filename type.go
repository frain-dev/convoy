package hookcamp

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HttpMethod string

type Period int

type DBTime primitive.DateTime

var PeriodValues = map[string]Period{
	"daily":   Daily,
	"weekly":  Weekly,
	"monthly": Monthly,
	"yearly":  Yearly,
}

func IsValidPeriod(period string) bool {
	_, ok := PeriodValues[period]
	return ok
}
