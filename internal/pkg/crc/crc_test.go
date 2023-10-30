package crc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func Test_TwitterCrc_HandleRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sourceRepo := mocks.NewMockSourceRepository(ctrl)

	tests := map[string]struct {
		secret    string
		requestFn func(t *testing.T) (*httptest.ResponseRecorder, *http.Request)
		source    *datastore.Source
		dbFn      func(so *mocks.MockSourceRepository)
		wantToken string
	}{
		"valid_token": {
			secret: "Convoy",
			requestFn: func(t *testing.T) (*httptest.ResponseRecorder, *http.Request) {
				req, err := http.NewRequest("GET", "URL?crc_token=uzwcfYtzr9", nil)
				require.NoError(t, err)

				w := httptest.NewRecorder()
				return w, req
			},
			source: &datastore.Source{
				UID:       "123",
				ProjectID: "abc",
				ProviderConfig: &datastore.ProviderConfig{
					Twitter: &datastore.TwitterProviderConfig{},
				},
			},
			dbFn: func(so *mocks.MockSourceRepository) {
				so.EXPECT().UpdateSource(gomock.Any(), gomock.Any()).Return(nil)
			},
			wantToken: "sha256=HXvxTdsfShG6k2zC9NVANwFquJBdOugRYHax2vNiiOo=",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := NewTwitterCrc(tc.secret)
			w, r := tc.requestFn(t)

			if tc.dbFn != nil {
				tc.dbFn(sourceRepo)
			}

			err := c.HandleRequest(w, r, tc.source, sourceRepo)
			require.NoError(t, err)

			var response TwitterCrcResponse
			err = json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, tc.wantToken, response.ResponseToken)
		})
	}
}
