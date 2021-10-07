package util

import (
	"net/http"
	"strings"

	"github.com/frain-dev/convoy"
)

// ConvertDefaultHeaderToCustomHeader converts http.Header to convoy.HttpHeader
func ConvertDefaultHeaderToCustomHeader(h *http.Header) *convoy.HttpHeader {
	res := make(convoy.HttpHeader, 0)
	for k, v := range *h {
		res[k] = strings.Join(v, " ")
	}

	return &res
}
