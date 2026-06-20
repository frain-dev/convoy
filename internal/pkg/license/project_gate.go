package license

import "errors"

var ErrProjectDisabled = errors.New("this project has been disabled for write operations until you re-subscribe your convoy instance")

func EnsureProjectEnabled(licenser Licenser, projectID string) error {
	if !licenser.ProjectEnabled(projectID) {
		return ErrProjectDisabled
	}
	return nil
}
