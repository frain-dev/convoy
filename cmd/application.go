package main

import "github.com/spf13/cobra"

func addApplicationCommnand(a *app) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "application",
		Aliases: []string{"app"},
		Short:   "Manage applications",
	}

	return cmd
}

func createApplication(a *app) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an application",
		RunE: func(cmd *cobra.Command, args []string) error {

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "The name of the application")

	return cmd
}
