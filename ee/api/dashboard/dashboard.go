package dashboard

import (
	base "github.com/frain-dev/convoy/api/dashboard"
	"github.com/frain-dev/convoy/api/types"
)

type DashboardHandler struct {
	*base.DashboardHandler
	Opts *types.APIOptions
}
