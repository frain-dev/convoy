package config

type OrganisationFetchMode string

const (
	FileSystemOrganisationFetchMode OrganisationFetchMode = "file"
	// Not supported yet though
	DashboardOrganisationFetchMode OrganisationFetchMode = "dashboard"
)

type Configuration struct {
	Organisation struct {
		FetchMode OrganisationFetchMode `json:"fetch_mode"`
		// This is only needed if FileSystemOrganisationFetchMode is
		// used
		FilePath string `json:"file_path"`
	} `json:"organisation"`
}
