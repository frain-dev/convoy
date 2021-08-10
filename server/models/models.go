package models

type Application struct {
	AppName string `json:"name" bson:"name"`
}
