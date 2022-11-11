package policies

import (
	"context"
	"testing"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/stretchr/testify/require"
)

func Test_OrganisationPolicy_Update(t *testing.T) {
}

func Test_OrganisationPolicy_Delete(t *testing.T) {
	tests := map[string]struct {
		authCtx       auth.AuthenticatedUser
		organization  *datastore.Organisation
		wantErr       bool
		expectedError error
	}{}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange.
			policy := &OrganisationPolicy{}
			authCtx := context.WithValue(context.Background(), AuthCtxKey, tc.authCtx)

			// Act.
			err := policy.Delete(authCtx, tc.organization)

			// Assert.
			if tc.wantErr {
				require.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
		})
	}
}
