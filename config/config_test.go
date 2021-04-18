package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOrganisationFetchMode(t *testing.T) {

	tt := []struct {
		o     OrganisationFetchMode
		valid bool
	}{
		{FileSystemOrganisationFetchMode, true},
		{DashboardOrganisationFetchMode, false},
		{OrganisationFetchMode("oops"), false},
	}

	for _, v := range tt {
		err := v.o.Validate()
		if v.valid {
			require.NoError(t, err)
			continue
		}

		require.Error(t, err)
	}
}
