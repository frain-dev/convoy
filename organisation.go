package hookstack

type Organisation struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token Token  `json:"token"`
}

type OrganisationRepository interface {
	// LoadOrganisations fetches all known organisations
	// This is needed because we want to support both headless mode - from a
	// flat file - and also from a database
	LoadOrganisations() ([]Organisation, error)
}
