package license

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/internal/pkg/license/noop"
)

type disabledProjectLicenser struct {
	*noop.Licenser
}

func (disabledProjectLicenser) ProjectEnabled(string) bool {
	return false
}

func TestEnsureProjectEnabled(t *testing.T) {
	require.NoError(t, EnsureProjectEnabled(noop.NewLicenser(), "project-id"))
	require.ErrorIs(t, EnsureProjectEnabled(disabledProjectLicenser{Licenser: noop.NewLicenser()}, "project-id"), ErrProjectDisabled)
}
