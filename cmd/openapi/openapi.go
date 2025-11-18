package openapi

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/cobra"

	"github.com/frain-dev/convoy/internal/pkg/openapi"
)

func AddOpenAPICommand() *cobra.Command {
	var (
		inputFile  string
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "openapi",
		Short: "Extract webhook schemas from OpenAPI specifications",
		Long: `Extract webhook schemas from OpenAPI 2.x and 3.x specifications and convert them to JSON Schema format.
This command helps you identify webhook endpoints in your OpenAPI spec and generate corresponding JSON schemas.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inputFile == "" {
				return fmt.Errorf("input file is required")
			}

			// Read the file content
			content, err := os.ReadFile(inputFile)
			if err != nil {
				return fmt.Errorf("error reading OpenAPI spec: %v", err)
			}

			// Load as OpenAPI 3.x
			loader := openapi3.NewLoader()
			loader.IsExternalRefsAllowed = true
			swagger, err := loader.LoadFromData(content)
			if err != nil {
				return fmt.Errorf("error loading OpenAPI 3.x spec: %v", err)
			}

			// Create converter and extract webhooks
			conv, err := openapi.New(swagger)
			if err != nil {
				return fmt.Errorf("error creating converter: %v", err)
			}

			collection, err := conv.ExtractWebhooks()
			if err != nil {
				return fmt.Errorf("error extracting webhooks: %v", err)
			}

			// Write output
			output, err := json.MarshalIndent(collection, "", "  ")
			if err != nil {
				return fmt.Errorf("error marshaling output: %v", err)
			}

			if outputFile == "" {
				fmt.Printf("Successfully extracted %d webhook schemas. See below:\n%s", len(collection.Webhooks), output)
				return nil
			}

			err = os.WriteFile(outputFile, output, 0644)
			if err != nil {
				return fmt.Errorf("error writing output file: %v", err)
			}

			fmt.Printf("Successfully extracted %d webhook schemas to %s\n", len(collection.Webhooks), outputFile)
			return nil
		},
	}

	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Path to OpenAPI specification file (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Path to output JSON Schema file")

	_ = cmd.MarkFlagRequired("input")

	return cmd
}
