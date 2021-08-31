package hookcamp

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOrganisation_IsDeleted(t *testing.T) {

	tt := []struct {
		name         string
		organisation *Organisation
		isDeleted    bool
	}{
		{
			name:         "set deleted_at to zero",
			organisation: &Organisation{UID: "123456", DeletedAt: 0},
		},
		{
			name:         "skip deleted_at field",
			organisation: &Organisation{UID: "123456"},
		},
		{
			name:         "set deleted_at to random integer",
			organisation: &Organisation{UID: "123456", DeletedAt: 39487},
			isDeleted:    true,
		},
		{
			name:         "set deleted_at to current timestamp",
			organisation: &Organisation{UID: "123456", DeletedAt: primitive.NewDateTimeFromTime(time.Now())},
			isDeleted:    true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.isDeleted, tc.organisation.IsDeleted())
		})
	}
}

func TestOrganisation_IsOwner(t *testing.T) {

	tt := []struct {
		name         string
		organisation *Organisation
		app          *Application
		isOwner      bool
	}{
		{
			name:         "right owner",
			organisation: &Organisation{UID: "123456", DeletedAt: 0},
			app:          &Application{OrgID: "123456"},
			isOwner:      true,
		},
		{
			name:         "wrong owner",
			organisation: &Organisation{UID: "123456", DeletedAt: 0},
			app:          &Application{OrgID: "1234567"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.isOwner, tc.organisation.IsOwner(tc.app))
		})
	}
}
