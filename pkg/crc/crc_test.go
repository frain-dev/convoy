package crc

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_TwitterCrc_ValidateRequest(t *testing.T) {
	tests := map[string]struct {
		secret         string
		requestFn      func(t *testing.T) *http.Request
		encryptedToken string
	}{
		"valid_token": {
			secret: "Convoy",
			requestFn: func(t *testing.T) *http.Request {
				req, err := http.NewRequest("GET", "URL?crc_token=uzwcfYtzr9", nil)
				require.NoError(t, err)
				return req
			},
			encryptedToken: "sha256=HXvxTdsfShG6k2zC9NVANwFquJBdOugRYHax2vNiiOo=",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewTwitterCrc(tc.secret)
			req := tc.requestFn(t)

			res := c.ValidateRequest(req)

			require.Equal(t, tc.encryptedToken, res["response_token"])
		})
	}
}
