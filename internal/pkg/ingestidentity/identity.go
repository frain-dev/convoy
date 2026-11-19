package ingestidentity

import (
	"net/http"

	"github.com/google/uuid"
)

// DeliveryID reads the source's configured GUID header before a database lookup.
func DeliveryID(headers map[string]string, maskID string, request *http.Request) string {
	header := headers[maskID]
	if header == "" {
		return ""
	}
	value := request.Header.Get(header)
	if len(value) != 36 {
		return ""
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return ""
	}
	return id.String()
}
