package main

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"

	"github.com/hookstack/hookstack"
	"github.com/hookstack/hookstack/config"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

func addOrganisationCommnad() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "org",
		Short: "Manage organisations",
	}

	cmd.AddCommand(listOrganisationCommand())
	return cmd
}

func listOrganisationCommand() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all organisations currently known",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Get()
			if err != nil {
				return err
			}

			if err := cfg.Organisation.FetchMode.Validate(); err != nil {
				return err
			}

			if cfg.Organisation.FetchMode == config.FileSystemOrganisationFetchMode {

				f, err := os.Open(cfg.Organisation.FilePath)
				if err != nil {
					return err
				}

				defer f.Close()

				var orgs []hookstack.Organisation

				if err := json.NewDecoder(f).Decode(&orgs); err != nil {
					return err
				}

				table := tablewriter.NewWriter(os.Stdout)
				table.SetHeader([]string{"Counter", "ID", "Name", "Token"})

				for v, org := range orgs {
					table.Append([]string{strconv.Itoa(v + 1), org.ID, org.Name, org.Token.String()})
				}

				table.Render()
				return nil
			}

			return errors.New("unsupported fetch mode")
		},
	}

	return cmd
}
