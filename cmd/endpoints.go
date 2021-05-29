package main

import (
	"errors"

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
