package datastore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_FilterBy(t *testing.T) {
	type Args struct {
		name     string
		expected string
		filter   FilterBy
	}

	args := []Args{
		{
			name:     "complete_filter",
			expected: "project_id:=uid-1 && created_at:[0..1] && app_id:=app-1",
			filter: FilterBy{
				EndpointID: "app-1",
				ProjectID:  "uid-1",
				SearchParams: SearchParams{
					CreatedAtStart: 0,
					CreatedAtEnd:   1,
				},
			},
		},
		{
			name:     "missing_app_id",
			expected: "project_id:=uid-1 && created_at:[0..1]",
			filter: FilterBy{
				ProjectID: "uid-1",
				SearchParams: SearchParams{
					CreatedAtStart: 0,
					CreatedAtEnd:   1,
				},
			},
		},
	}

	for _, tt := range args {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.filter.String()
			require.Equal(t, tt.expected, *s)
		})
	}
}
