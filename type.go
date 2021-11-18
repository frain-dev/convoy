package convoy

import "strings"

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

func (t TaskName) SetPrefix(prefix string) TaskName {
	var name strings.Builder
	delim := "-"

	name.WriteString(prefix)
	name.WriteString(delim)
	name.WriteString(string(t))

	return TaskName(name.String())
}

const (
	EventProcessor      TaskName = "EventProcessor"
	DeadLetterProcessor TaskName = "DeadLetterProcessor"
)
