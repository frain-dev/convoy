package hookcamp

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Application defines an entity that can receive webhooks.
type Application struct {
	ID    uuid.UUID `json:"id" gorm:"type:uuid;uniqueIndex;not null"`
	OrgID uuid.UUID `json:"org_id" gorm:"not null"`
	Title string    `json:"name" gorm:"not null;type:varchar(200)"`

	gorm.Model
	Organisation Organisation `json:"organisation" gorm:"foreignKey:OrgID"`
}

// Endpoint defines a target service that can be reached in an application
type Endpoint struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;uniqueIndex;not null"`
	AppID       uuid.UUID `json:"app_id" gorm:"size:200;not null"`
	TargetURL   string    `json:"target_url" gorm:"not null"`
	Secret      string    `json:"secret" gorm:"type:varchar(200);uniqueIndex;not null"`
	Description string    `json:"description" gorm:"size:220;default:''"`

	Application Application `json:"-" gorm:"foreignKey:AppID"`
	gorm.Model
}

// ApplicationRepository is an abstraction over all database operations of an
// application
type ApplicationRepository interface {
	// CreateApplication when called persists an application to the database
	CreateApplication(context.Context, *Application) error

	// LoadApplications fetches a list of all apps from the database
	LoadApplications(context.Context) ([]Application, error)
}

// type EndpointRepository interface {
// 	CreateEndpoint(context.Context, *Endpoint) error
// }
