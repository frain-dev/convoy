package convoy

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/config"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrGroupNotFound = errors.New("group not found")

type Group struct {
	ID         primitive.ObjectID  `json:"-" bson:"_id"`
	UID        string              `json:"uid" bson:"uid"`
	Name       string              `json:"name" bson:"name"`
	LogoURL    string              `json:"logo_url" bson:"logo_url"`
	Config     *config.GroupConfig `json:"config" bson:"config"`
	Statistics *GroupStatistics    `json:"statistics" bson:"-"`

	CreatedAt primitive.DateTime `json:"created_at,omitempty" bson:"created_at,omitempty" swaggertype:"string"`
	UpdatedAt primitive.DateTime `json:"updated_at,omitempty" bson:"updated_at,omitempty" swaggertype:"string"`
	DeletedAt primitive.DateTime `json:"deleted_at,omitempty" bson:"deleted_at,omitempty" swaggertype:"string"`

	DocumentStatus DocumentStatus `json:"-" bson:"document_status"`
}

type GroupStatistics struct {
	MessagesSent int64 `json:"messages_sent"`
	TotalApps    int64 `json:"total_apps"`
}

type GroupFilter struct {
	Names []string `json:"name" bson:"name"`
}

func (o *Group) IsDeleted() bool { return o.DeletedAt > 0 }

func (o *Group) IsOwner(a *Application) bool { return o.UID == a.GroupID }

type GroupRepository interface {
	LoadGroups(context.Context, *GroupFilter) ([]*Group, error)
	CreateGroup(context.Context, *Group) error
	UpdateGroup(context.Context, *Group) error
	DeleteGroup(ctx context.Context, uid string) error
	FetchGroupByID(context.Context, string) (*Group, error)
}
