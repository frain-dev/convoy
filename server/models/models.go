package models

type Application struct {
	AppName string `json:"name" bson:"name"`
}

type Endpoint struct {
	URL         string `json:"url" bson:"url"`
	Secret      string `json:"secret" bson:"secret"`
	Description string `json:"description" bson:"description"`
}
