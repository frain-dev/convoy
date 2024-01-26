package v20240101

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/util"
	"github.com/jinzhu/copier"
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
		fmt.Println("Amen")
		return b, h, nil
	}

	var res []endpointResponse

	for _, endpointPayload := range endpoints {
		endpointBytes, err := json.Marshal(endpointPayload)
		if err != nil {
			return nil, nil, err
		}

		var endpoint models.EndpointResponse
		err = json.Unmarshal(endpointBytes, &endpoint)
		if err != nil {
			return nil, nil, err
		}

		httpTimeout := endpoint.HttpTimeout
		rateLimitDuration := endpoint.RateLimitDuration

		var oldEndpointBody oldEndpoint
		err = copier.Copy(&oldEndpointBody, &endpoint.Endpoint)
		if err != nil {
			return nil, nil, err
		}

		// set timeout
		oldEndpointBody.HttpTimeout, err = transformIntToDurationString(httpTimeout)
		if err != nil {
			return nil, nil, err
		}

		oldEndpointBody.RateLimitDuration, err = transformIntToDurationString(rateLimitDuration)
		if err != nil {
			return nil, nil, err
		}

		res = append(res, endpointResponse{&oldEndpointBody})
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
