package handlers

import (
	"testing"
	"time"

	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
)

func TestIsOrganisationDisabled(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name string
		org  *datastore.Organisation
		want bool
	}{
		{
			name: "disabled_at null value",
			org: &datastore.Organisation{
				DisabledAt: null.Time{},
			},
			want: false,
		},
		{
			name: "disabled_at set to timestamp",
			org: &datastore.Organisation{
				DisabledAt: null.NewTime(time.Now(), true),
			},
			want: true,
		},
		{
			name: "disabled_at zero-time but valid true",
			org: &datastore.Organisation{
				DisabledAt: null.NewTime(time.Time{}, true),
			},
			want: false,
		},
		{
			name: "disabled_at epoch exactly",
			org: &datastore.Organisation{
				DisabledAt: null.NewTime(time.Unix(0, 0), true),
			},
			want: false,
		},
		{
			name: "disabled_at before epoch",
			org: &datastore.Organisation{
				DisabledAt: null.NewTime(time.Unix(-1, 0), true),
			},
			want: false,
		},
		{
			name: "disabled_at timestamp but valid false",
			org: &datastore.Organisation{
				DisabledAt: null.NewTime(time.Now(), false),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.isOrganisationDisabled(tt.org)
			if got != tt.want {
				t.Fatalf("isOrganisationDisabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

