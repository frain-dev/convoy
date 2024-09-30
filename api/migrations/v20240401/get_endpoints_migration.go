package v20240401

import (
	"encoding/json"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/util"
)

type GetEndpointsResponseMigration struct{}

func (g *GetEndpointsResponseMigration) Migrate(b []byte, h http.Header) ([]byte, http.Header, error) {
	var serverResponse util.ServerResponse
	err := json.Unmarshal(b, &serverResponse)
	if err != nil {
		return nil, nil, err
	}

	var pResp models.PagedResponse
	err = json.Unmarshal(serverResponse.Data, &pResp)
	if err != nil {
		return nil, nil, err
	}

	if pResp.Content == nil {
		// nothing to transform.
		return b, h, nil
	}

	endpoints, ok := pResp.Content.([]any)
	if !ok {
		// invalid type.
		return b, h, nil
	}

	var res []oldEndpointResponse

	for _, endpointPayload := range endpoints {
		endpointBytes, err := json.Marshal(endpointPayload)
		if err != nil {
			return nil, nil, err
		}

		var endpointResp models.EndpointResponse
		err = json.Unmarshal(endpointBytes, &endpointResp)
		if err != nil {
			return nil, nil, err
		}

		var old oldEndpointResponse
		endpoint := endpointResp.Endpoint

		err = migrateEndpoint(&endpoint, &old)
		if err != nil {
			return nil, nil, err
		}

		res = append(res, old)
	}

	pResp.Content = res
	b, err = json.Marshal(pResp)
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
