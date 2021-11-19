package config

import (
	"fmt"

	"github.com/frain-dev/convoy/auth"
)

type Role = auth.Role

func checkRole(role *Role, credType string) error {
	if !role.Type.IsValid() {
		return fmt.Errorf("invalid role type: %s", role.Type.String())
	}

	// groups will never be checked for superusers
	if len(role.Groups) == 0 && !role.Type.Is(auth.RoleSuperUser) {
		return fmt.Errorf("please specify groups for %s", credType)
	}

	for _, group := range role.Groups {
		if group == "" {
			return fmt.Errorf("empty group name not allowed for %s", credType)
		}
	}
	return nil
}
