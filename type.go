package convoy

type HttpMethod string

type DocumentStatus string

const (
	ActiveDocumentStatus   DocumentStatus = "Active"
	InactiveDocumentStatus DocumentStatus = "Inactive"
	DeletedDocumentStatus  DocumentStatus = "Deleted"
)

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

type TaskName string

const (
	EventProcessor         TaskName = "EventProcessor"
	EventDeliveryProcessor TaskName = "EventDeliveryProcessor"
	DeadLetterProcessor    TaskName = "DeadLetterProcessor"
)
