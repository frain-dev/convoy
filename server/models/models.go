package models

type Organisation struct {
	Name string `json:"name" bson:"name"`
}

type Application struct {
	OrgID   string `json:"orgId" bson:"orgId"`
	AppName string `json:"name" bson:"name"`
}

type Endpoint struct {
	URL         string `json:"url" bson:"url"`
	Secret      string `json:"secret" bson:"secret"`
	Description string `json:"description" bson:"description"`
}

type Pageable struct {
	Page    int `json:"page" bson:"page"`
	PerPage int `json:"perPage" bson:"perPage"`
}

type SearchParams struct {
	CreatedAtStart int64 `json:"createdAtStart" bson:"created_at_start"`
	CreatedAtEnd   int64 `json:"createdAtEnd" bson:"created_at_end"`
}

type DashboardSummary struct {
	MessagesSent int           `json:"messages" bson:"messages"`
	Applications int           `json:"apps" bson:"apps"`
	MessageData  []MessageData `json:"messageData" bson:"message_data"`
}

type MessageData struct {
	Day   int `json:"day" bson:"day"`
	Count int `json:"count" bson:"count"`
}
