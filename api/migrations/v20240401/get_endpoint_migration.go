package v20240401

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/util"
)

type GetEndpointResponseMigration struct{}

func (c *GetEndpointResponseMigration) Migrate(b []byte, h http.Header) ([]byte, http.Header, error) {
	var serverResponse util.ServerResponse
	err := json.Unmarshal(b, &serverResponse)
	if err != nil {
		return nil, nil, err
	}

	if len(serverResponse.Data) == 0 {
		// nothing to transform.
		return b, h, nil
	}

	var endpointResp *models.EndpointResponse
	err = json.Unmarshal(serverResponse.Data, &endpointResp)
	if err != nil {
		return nil, nil, err
	}

	endpoint := endpointResp.Endpoint

	var old OldEndpointResponse
	err = migrateEndpoint(&endpoint, &old)
	if err != nil {
		fmt.Printf("err: %+v\n", err)
		return nil, nil, err
	}

	b, err = json.Marshal(old)
	if err != nil {
		fmt.Printf("err2: %+v\n", err)
		return nil, nil, err
	}

	serverResponse.Data = b

	sb, err := json.Marshal(serverResponse)
	if err != nil {
		fmt.Printf("err3: %+v\n", err)
		return nil, nil, err
	}

	return sb, h, nil
}
