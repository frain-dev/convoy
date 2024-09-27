package services

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/dop251/goja_nodejs/process"

	"github.com/frain-dev/convoy"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
)

type CreateOpenAPISpecCatalogueService struct {
	CatalogueRepo        datastore.EventCatalogueRepository
	EventRepo            datastore.EventRepository
	CatalogueOpenAPISpec *models.CatalogueOpenAPISpec
	Project              *datastore.Project
}

func (c *CreateOpenAPISpecCatalogueService) Run(ctx context.Context) (*datastore.EventCatalogue, error) {
	if c.Project.Type != datastore.OutgoingProject {
		return nil, &ServiceError{ErrMsg: "event catalogue is only available to outgoing projects"}
	}

	fmt.Println("444444")

	_, err := getRuntime(c.CatalogueOpenAPISpec.OpenAPISpec)
	if err != nil {
		fmt.Println("errrr", err)
		return nil, err
	}
	//catalogue := &datastore.EventCatalogue{
	//	UID:         ulid.Make().String(),
	//	ProjectID:   c.Project.UID,
	//	Type:        datastore.OpenAPICatalogueType,
	//	OpenAPISpec: c.CatalogueOpenAPISpec.OpenAPISpec,
	//	CreatedAt:   time.Now(),
	//	UpdatedAt:   time.Now(),
	//}

	// err = c.CatalogueRepo.CreateEventCatalogue(ctx, catalogue)
	if err != nil {
		return nil, &ServiceError{
			ErrMsg: "failed to create open api spec catalogue",
			Err:    err,
		}
	}

	return nil, err
}

func getRuntime(openapiSpec string) (*goja.Runtime, error) {
	rt := goja.New()
	req := new(require.Registry).Enable(rt)

	require.RegisterCoreModule(process.ModuleName, process.Require)

	process.Enable(rt)

	files, err := convoy.JSFiles.ReadDir("js")
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() {
			// b, err := convoy.JSFiles.ReadFile(file.Name())

			v, err := req.Require("./js/" + file.Name())
			//_, err = rt.RunString(string(b))
			if err != nil {
				return nil, fmt.Errorf("rewtwtre %v", err)
			}

			fmt.Println("jsvalue", v.Export())
		}
	}

	dec := make([]byte, base64.RawURLEncoding.DecodedLen(len(openapiSpec)))
	_, err = base64.RawURLEncoding.Decode(dec, []byte(openapiSpec))
	if err != nil {
		return nil, err
	}

	_ = rt.Set("openApiFilePath", string(dec))

	v, err := rt.RunString(`


let openApiFilePath = null;
function processOpenAPI(openApiFilePath) {

    try {
        const dereferencedData = await dereference(load(openApiFilePath));
        var final = [];

        Object.entries(dereferencedData.webhooks).forEach(([key, value]) => {

            let schema = structuredClone(value.post.requestBody.content['application/json'].schema);

            schema['description'] = value.post.requestBody.description
            schema['sample_json'] = sample(schema)
            schema['name'] = key

            delete schema.required
            final.push(schema);
        });

        return final;
    } catch (error) {
        console.error('Error processing OpenAPI spec:', error);
        return null;
    }
}`)
	if err != nil {
		fmt.Println("9999", err)
		return nil, err
	}

	fmt.Println("111111", v.String())

	return rt, nil
}
