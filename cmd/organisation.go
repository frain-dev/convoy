package main

import (
	"fmt"

	"github.com/hookstack/hookstack/config"
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
			fmt.Println("All organisations")
			fmt.Println(config.Get())
			return nil
		},
	}

	return cmd
}
