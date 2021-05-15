package hookcamp

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Application defines an entity that can receive webhooks.
type Application struct {
	ID    uuid.UUID `json:"id" gorm:"type:uuid;uniqueIndex,not null"`
	OrgID uuid.UUID `json:"org_id" gorm:"not null"`

	gorm.Model
	Organisation Organisation `json:"organisation" gorm:"foreignKey:OrgID"`
}

// Endpoint defines a target service that can be reached in an application
type Endpoint struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;uniqueIndex,not null"`
	AppID       uuid.UUID `json:"app_id" gorm:"size:200;not null"`
	TargetURL   string    `json:"target_url" gorm:"not null"`
	Secret      string    `json:"secret" gorm:"type:varchar(200);index:idx_secret; unique, not null"`
	Description string    `json:"description" gorm:"size:220;default:''"`

	Application Application `json:"-" gorm:"foreignKey:AppID"`
	gorm.Model
}
