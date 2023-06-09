package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/frain-dev/convoy/pkg/dedup"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"time"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/internal/pkg/cli"
	"github.com/frain-dev/convoy/internal/pkg/migrator"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/spf13/cobra"
)

func AddMigrateCommand(a *cli.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Convoy migrations",
	}

	cmd.AddCommand(addUpCommand())
	cmd.AddCommand(addDownCommand())
	cmd.AddCommand(addCreateCommand())
	cmd.AddCommand(addRunCommand())

	return cmd
}

func addUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "up",
		Aliases: []string{"migrate-up"},
		Short:   "Run all pending migrations",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Fatalf("Error fetching the config.")
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}

			defer db.Close()

			m := migrator.New(db)
			err = m.Up()
			if err != nil {
				log.Fatalf("migration up failed with error: %+v", err)
			}
		},
	}

	return cmd
}

func addDownCommand() *cobra.Command {
	var max int

	cmd := &cobra.Command{
		Use:     "down",
		Aliases: []string{"migrate-down"},
		Short:   "Rollback migrations",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.Get()
			if err != nil {
				log.WithError(err).Fatalf("Error fetching the config.")
			}

			db, err := postgres.NewDB(cfg)
			if err != nil {
				log.Fatal(err)
			}

			defer db.Close()

			m := migrator.New(db)
			err = m.Down(max)
			if err != nil {
				log.Fatalf("migration down failed with error: %+v", err)
			}
		},
	}

	cmd.Flags().IntVar(&max, "max", 1, "The maximum number of migrations to rollback")

	return cmd
}

func addCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "creates a new migration file",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			fileName := fmt.Sprintf("sql/%v.sql", time.Now().Unix())
			f, err := os.Create(fileName)
			if err != nil {
				log.Fatal(err)
			}

			defer f.Close()

			lines := []string{"-- +migrate Up", "-- +migrate Down"}
			for _, line := range lines {
				_, err := f.WriteString(line + "\n\n")
				if err != nil {
					log.Fatal(err)
				}
			}
		},
	}

	return cmd
}

func addRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "creates a new migration file",
		Annotations: map[string]string{
			"CheckMigration":  "false",
			"ShouldBootstrap": "false",
		},
		Run: func(cmd *cobra.Command, args []string) {
			header()
			requestBody()
			requestBodyFormData()
			requestBodyFormDataNested()
			queryParam()
			requestBodyUrlEncoded()
		},
	}

	return cmd
}

func queryParam() {
	////////////////////////////////////////////////////////////////////////////
	//
	// QueryParam
	//
	////////////////////////////////////////////////////////////////////////////

	request, err := http.NewRequest("GET", "https://example.com/?name=John&age=25", nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	duper, err := dedup.NewDeDuper(context.Background(), "redis://localhost:6379", request)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	err = duper.Set("", []string{"request.QueryParam.name"}, time.Minute)
	if err != nil {
		fmt.Println("Error setting data:", err)
		return
	}

	result, err := duper.Get("", []string{"request.QueryParam.name"})
	if err != nil {
		fmt.Println("Error fetching data:", err)
		return
	}

	fmt.Println(result) // Output: John
}

func header() {
	////////////////////////////////////////////////////////////////////////////
	//
	// Header
	//
	////////////////////////////////////////////////////////////////////////////

	// Example for extracting data from the request header
	requestHeader := http.Header{}
	requestHeader.Add("Authorization", "Bearer myToken123")
	request, err := http.NewRequest("GET", "https://example.com", nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header = requestHeader

	duper, err := dedup.NewDeDuper(context.Background(), "redis://localhost:6379", request)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	err = duper.Set("noop", []string{"request.Header.Authorization"}, time.Minute)
	if err != nil {
		fmt.Println("Error setting data:", err)
		return
	}

	result, err := duper.Get("noop", []string{"request.Header.Authorization"})
	if err != nil {
		fmt.Println("Error fecthing data:", err)
		return
	}

	fmt.Println(result) // Output: Bearer myToken123
}

func requestBody() {
	////////////////////////////////////////////////////////////////////////////
	//
	// Body (JSON)
	//
	////////////////////////////////////////////////////////////////////////////

	// Example for extracting data from the request body
	makeRequest := func() *http.Request {
		person := struct {
			Age int `json:"age"`
		}{
			Age: 25,
		}
		body, err := json.Marshal(person)
		if err != nil {
			fmt.Println("Error marshaling JSON:", err)
		}
		request, err := http.NewRequest("POST", "https://example.com", strings.NewReader(string(body)))
		if err != nil {
			fmt.Println("Error creating request:", err)
		}
		request.Header.Set("Content-Type", "application/json")
		return request
	}

	duper, err := dedup.NewDeDuper(context.Background(), "redis://localhost:6379", makeRequest())
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	err = duper.Set("", []string{"request.Body.age"}, time.Minute*60)
	if err != nil {
		fmt.Println("Error setting data:", err)
		return
	}

	d, err := dedup.NewDeDuper(context.Background(), "redis://localhost:6379", makeRequest())
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	result, err := d.Get("", []string{"request.Body.age"})
	if err != nil {
		fmt.Println("Error fetching data:", err)
		return
	}

	fmt.Println(result) // Output: 25
}

func requestBodyFormData() {
	////////////////////////////////////////////////////////////////////////////
	//
	// Body (Form Data)
	//
	////////////////////////////////////////////////////////////////////////////

	// Example for extracting data from a form-data request body
	makeRequest := func() *http.Request {
		b := &bytes.Buffer{}
		writer := multipart.NewWriter(b)
		err := writer.WriteField("name", "John")
		if err != nil {
			fmt.Println("Error creating request:", err)
		}
		err = writer.Close()
		if err != nil {
			fmt.Println("Error creating request:", err)
		}

		request, err := http.NewRequest("POST", "https://example.com", b)
		if err != nil {
			fmt.Println("Error creating request:", err)
		}
		request.Header.Set("Content-Type", writer.FormDataContentType())
		return request
	}

	duper, err := dedup.NewDeDuper(context.Background(), "redis://localhost:6379", makeRequest())
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	err = duper.Set("", []string{"request.body.name"}, time.Minute)
	if err != nil {
		fmt.Println("Error extracting data:", err)
		return
	}

	d, err := dedup.NewDeDuper(context.Background(), "redis://localhost:6379", makeRequest())
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	result, err := d.Get("", []string{"request.body.name"})
	if err != nil {
		fmt.Println("Error extracting data:", err)
		return
	}

	fmt.Println(result) // Output: John
}

func requestBodyUrlEncoded() {
	////////////////////////////////////////////////////////////////////////////
	//
	// Body (Url Encoded)
	//
	////////////////////////////////////////////////////////////////////////////

	// Example for extracting data from an x-www-form-urlencoded request body
	request, err := http.NewRequest("POST", "https://example.com", strings.NewReader("age=25"))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	duper, err := dedup.NewDeDuper(context.Background(), "redis://localhost:6379", request)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	err = duper.Set("", []string{"request.Body.age"}, time.Minute)
	if err != nil {
		fmt.Println("Error setting data:", err)
		return
	}

	result, err := duper.Get("", []string{"request.Body.age"})
	if err != nil {
		fmt.Println("Error fetching data:", err)
		return
	}

	fmt.Println(result) // Output: 25
}

func requestBodyFormDataNested() {
	type Person struct {
		Name    string
		Age     int
		Address struct {
			Street string
			City   string
			Zip    string
		}
	}

	// Create a new person with nested data
	person := Person{
		Name: "John",
		Age:  25,
		Address: struct {
			Street string
			City   string
			Zip    string
		}{
			Street: "123 Main St",
			City:   "New York",
			Zip:    "10001",
		},
	}

	// Create a buffer to store the form data
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	// Set the Content-Type header
	contentType := writer.FormDataContentType()

	// Write the person's name
	fieldName := "name"
	fieldValue := person.Name
	writer.WriteField(fieldName, fieldValue)

	// Write the person's age
	fieldName = "age"
	fieldValue = fmt.Sprintf("%d", person.Age)
	writer.WriteField(fieldName, fieldValue)

	// Write the address fields
	addressPrefix := "address"
	addressFields := []struct {
		Name  string
		Value string
	}{
		{Name: "street", Value: person.Address.Street},
		{Name: "city", Value: person.Address.City},
		{Name: "zip", Value: person.Address.Zip},
	}

	for _, field := range addressFields {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s[%s]"`, addressPrefix, field.Name))
		part, _ := writer.CreatePart(h)
		part.Write([]byte(field.Value))
	}

	// Close the multipart writer
	writer.Close()

	// Create a new HTTP request with the form data
	request, err := http.NewRequest("POST", "https://example.com", body)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	request.Header.Set("Content-Type", contentType)

	duper, err := dedup.NewDeDuper(context.Background(), "redis://localhost:6379", request)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	// Extract data from the request
	err = duper.Set("", []string{"request.Body.address[zip]", "request.Body.address[city]"}, time.Minute)
	if err != nil {
		fmt.Println("Error extracting data:", err)
		return
	}

	result, err := duper.Get("", []string{"request.Body.address[zip]", "request.Body.address[city]"})
	if err != nil {
		fmt.Println("Error extracting data:", err)
		return
	}

	fmt.Println(result) // Output: 10001
}
