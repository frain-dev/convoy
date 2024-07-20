package v20240101

import (
	"encoding/json"
	"net/http"

	v20240401 "github.com/frain-dev/convoy/api/migrations/v20240401"
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

	var endpointResp v20240401.OldEndpointResponse
	err = json.Unmarshal(serverResponse.Data, &endpointResp)
	if err != nil {
		return nil, nil, err
	}

	var oldEndpoint oldEndpoint
	err = migrateEndpoint(&endpointResp, &oldEndpoint, backward)
	if err != nil {
		return nil, nil, err
	}

	newEndpointResponse := &endpointResponse{&oldEndpoint}

	b, err = json.Marshal(newEndpointResponse)
	if err != nil {
		return nil, nil, err
	}

	serverResponse.Data = json.RawMessage(b)

	sb, err := json.Marshal(serverResponse)
	if err != nil {
		return nil, nil, err
	}

	return sb, h, nil
}
