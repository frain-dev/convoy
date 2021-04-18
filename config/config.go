package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

var cfgSingleton atomic.Value

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

func LoadFromFile(p string) error {

	f, err := os.Open(p)
	if err != nil {
		return err
	}

	defer f.Close()

	c := new(Configuration)

	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return err
	}

	cfgSingleton.Store(c)
	return nil
}

func Get() (Configuration, error) {
	c, ok := cfgSingleton.Load().(*Configuration)
	if !ok {
		return Configuration{}, errors.New("call Load before this function")
	}

	return *c, nil
}
