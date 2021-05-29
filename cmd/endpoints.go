package main

import (
	"errors"

	"github.com/hookcamp/hookcamp"
	"github.com/spf13/cobra"
)

func addEndpointCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "endpoint",
		Aliases: []string{"e"},
		Short:   "Manage application endpoints",
	}

	cmd.AddCommand(createEndpointCommand(a))
	cmd.AddCommand(getEndpointCommand(a))

	return cmd
}

func getEndpointCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get the details of an endpoint",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("requires an ID argument")
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// ID := args[0]

			// endpointID, err := uuid.Parse(ID)
			// if err != nil {
			// 	return fmt.Errorf("Please provide a valid ID..%w", err)
			// }

			// ctx, cancelFn := getCtx()
			// defer cancelFn()

			// e, err := a.endpointRepo.FindEndpointByID(ctx, endpointID)
			// if err != nil {
			// 	return fmt.Errorf("could not fetch endpoint..%w", err)
			// }

			// table := tablewriter.NewWriter(os.Stdout)
			// table.SetHeader([]string{"ID", "Secret", "Target URL", "Description"})

			// table.Append([]string{e.ID.String(), e.Secret, e.TargetURL, e.Description})

			// table.Render()
			return nil
		},
	}

	return cmd
}

func createEndpointCommand(a *app) *cobra.Command {
	e := new(hookcamp.Endpoint)
	var appID string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			// var err error

			// if util.IsStringEmpty(e.Description) {
			// 	return errors.New("please provide a description")
			// }

			// if util.IsStringEmpty(e.Secret) {
			// 	e.Secret, err = util.GenerateRandomString(25)
			// 	if err != nil {
			// 		return fmt.Errorf("could not generate secret...%v", err)
			// 	}
			// }

			// if util.IsStringEmpty(e.TargetURL) {
			// 	return errors.New("please provide your target url")
			// }

			// u, err := url.Parse(e.TargetURL)
			// if err != nil {
			// 	return fmt.Errorf("please provide a valid url...%w", err)
			// }

			// e.TargetURL = u.String()

			// e.AppID, err = uuid.Parse(appID)
			// if err != nil {
			// 	return fmt.Errorf("please provide a valid app id..%w", err)
			// }

			// ctx, cancelFn := getCtx()
			// defer cancelFn()

			// _, err = a.applicationRepo.FindApplicationByID(ctx, e.AppID)
			// if err != nil {
			// 	return fmt.Errorf("could not fetch application from the database...%w", err)
			// }

			// ctx, cancelFn = getCtx()
			// defer cancelFn()

			// if err := a.endpointRepo.CreateEndpoint(ctx, e); err != nil {
			// 	return fmt.Errorf("could not create endpoint...%w", err)
			// }

			// fmt.Println("Endpoint was successfully created")
			// fmt.Println()

			// table := tablewriter.NewWriter(os.Stdout)
			// table.SetHeader([]string{"ID", "Secret", "Target URL", "Description"})

			// table.Append([]string{e.ID.String(), e.Secret, e.TargetURL, e.Description})

			// table.Render()
			return nil
		},
	}

	cmd.Flags().StringVar(&e.Description, "description", "", "Description of this endpoint")
	cmd.Flags().StringVar(&e.TargetURL, "target", "", "The target url of this endpoint")
	cmd.Flags().StringVar(&e.Secret, "secret", "",
		"Provide the secret for this endpoint. If blank, it will be automatically generated")
	cmd.Flags().StringVar(&appID, "app", "", "The app this endpoint belongs to")

	return cmd
}
