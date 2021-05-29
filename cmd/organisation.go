package main

import (
	"github.com/spf13/cobra"
)

func addOrganisationCommnad(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "org",
		Short: "Manage organisations",
	}

	cmd.AddCommand(listOrganisationCommand(a))
	cmd.AddCommand(createOrganisatonCommand(a))

	return cmd
}

func createOrganisatonCommand(a *app) *cobra.Command {
	var name string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an organisation",
		RunE: func(cmd *cobra.Command, args []string) error {
			// if util.IsStringEmpty(name) {
			// 	return errors.New("please provide the organisation name")
			// }

			// ctx, cancelFn := getCtx()
			// defer cancelFn()

			// org := &hookcamp.Organisation{
			// 	OrgName: name,
			// 	ID:      uuid.New(),
			// }

			// if err := a.orgRepo.CreateOrganisation(ctx, org); err != nil {
			// 	return err
			// }

			// fmt.Println("Your new organsation has been created")
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "The name of the organisation")

	return cmd
}

func listOrganisationCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all organisations",
		RunE: func(cmd *cobra.Command, args []string) error {
			// ctx, cancelFn := getCtx()
			// defer cancelFn()

			// orgs, err := a.orgRepo.LoadOrganisations(ctx)
			// if err != nil {
			// 	return err
			// }

			// table := tablewriter.NewWriter(os.Stdout)
			// table.SetHeader([]string{"ID", "Name", "Created at"})

			// for _, org := range orgs {
			// 	table.Append([]string{org.ID.String(), org.OrgName, org.CreatedAt.String()})
			// }

			// table.Render()
			return nil
		},
	}

	return cmd
}
