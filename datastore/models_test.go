package datastore

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/stretchr/testify/require"
)

func TestGroup_IsDeleted(t *testing.T) {

	tt := []struct {
		name      string
		group     *Group
		isDeleted bool
	}{
		{
			name:  "set deleted_at to zero",
			group: &Group{UID: "123456", DeletedAt: 0},
		},
		{
			name:  "skip deleted_at field",
			group: &Group{UID: "123456"},
		},
		{
			name:      "set deleted_at to random integer",
			group:     &Group{UID: "123456", DeletedAt: 39487},
			isDeleted: true,
		},
		{
			name:      "set deleted_at to current timestamp",
			group:     &Group{UID: "123456", DeletedAt: primitive.NewDateTimeFromTime(time.Now())},
			isDeleted: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.isDeleted, tc.group.IsDeleted())
		})
	}
}

func TestGroup_IsOwner(t *testing.T) {

	tt := []struct {
		name    string
		group   *Group
		app     *Application
		isOwner bool
	}{
		{
			name:    "right owner",
			group:   &Group{UID: "123456", DeletedAt: 0},
			app:     &Application{GroupID: "123456"},
			isOwner: true,
		},
		{
			name:  "wrong owner",
			group: &Group{UID: "123456", DeletedAt: 0},
			app:   &Application{GroupID: "1234567"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.isOwner, tc.group.IsOwner(tc.app))
		})
	}
}
