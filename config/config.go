package config

import (
	"fmt"
	"strings"
)

type OrganisationFetchMode string

const (
	FileSystemOrganisationFetchMode OrganisationFetchMode = "file"
	// Not supported yet though
	DashboardOrganisationFetchMode OrganisationFetchMode = "dashboard"
)

func (o OrganisationFetchMode) String() string { return strings.ToLower(string(o)) }

func (o OrganisationFetchMode) Validate() error {
	switch o {
	case FileSystemOrganisationFetchMode:
		return nil
	default:
		return fmt.Errorf("unkown org fetch mode (%s)", o)
	}
}

type Configuration struct {
	Organisation struct {
		FetchMode OrganisationFetchMode `json:"fetch_mode"`
		// This is only needed if FileSystemOrganisationFetchMode is
		// used
		FilePath string `json:"file_path"`
	} `json:"organisation"`
}
