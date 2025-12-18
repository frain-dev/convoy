package datastore

import (
	"net/http"
	"strings"
)

// ConvertDefaultHeaderToCustomHeader converts http.Header to convoy.HttpHeader
func ConvertDefaultHeaderToCustomHeader(h *http.Header) *HttpHeader {
	res := make(HttpHeader)
	for k, v := range *h {
		res[k] = strings.Join(v, " ")
	}

	return &res
}
