package hookstack

import (
	"encoding/json"
	"os"
)

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

func NewFileOrganisationLoader(p string) *FileOrganisationLoader {
	return &FileOrganisationLoader{pathToFile: p}
}

type FileOrganisationLoader struct {
	pathToFile string
}

func (fo *FileOrganisationLoader) LoadOrganisations() ([]Organisation, error) {
	var orgs []Organisation

	f, err := os.Open(fo.pathToFile)
	if err != nil {
		return orgs, err
	}

	defer f.Close()

	if err := json.NewDecoder(f).Decode(&orgs); err != nil {
		return orgs, err
	}

	return orgs, nil
}
