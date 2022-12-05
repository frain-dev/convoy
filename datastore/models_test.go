package datastore

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/stretchr/testify/require"
)

func TestProject_IsDeleted(t *testing.T) {
	d := primitive.DateTime(39487)
	deletedAt := primitive.NewDateTimeFromTime(time.Now())

	tt := []struct {
		name      string
		project   *Project
		isDeleted bool
	}{
		{
			name:    "set deleted_at to zero",
			project: &Project{UID: "123456", DeletedAt: nil},
		},
		{
			name:    "skip deleted_at field",
			project: &Project{UID: "123456"},
		},
		{
			name:      "set deleted_at to random integer",
			project:   &Project{UID: "123456", DeletedAt: &d},
			isDeleted: true,
		},
		{
			name:      "set deleted_at to current timestamp",
			project:   &Project{UID: "123456", DeletedAt: &deletedAt},
			isDeleted: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.isDeleted, tc.project.IsDeleted())
		})
	}
}

func TestProject_IsOwner(t *testing.T) {
	tt := []struct {
		name     string
		project  *Project
		endpoint *Endpoint
		isOwner  bool
	}{
		{
			name:     "right owner",
			project:  &Project{UID: "123456", DeletedAt: nil},
			endpoint: &Endpoint{ProjectID: "123456"},
			isOwner:  true,
		},
		{
			name:     "wrong owner",
			project:  &Project{UID: "123456", DeletedAt: nil},
			endpoint: &Endpoint{ProjectID: "1234567"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.isOwner, tc.project.IsOwner(tc.endpoint))
		})
	}
}
