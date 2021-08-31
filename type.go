package hookcamp

type HttpMethod string

type Period int

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
