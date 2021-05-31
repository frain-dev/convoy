package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/mocks"
)

func TestRequireAuth(t *testing.T) {

	org := &hookcamp.Organisation{
		UID:    uuid.New().String(),
		ApiKey: uuid.New().String(),
	}

	tt := []struct {
		name       string
		method     string
		statusCode int
		token      string
		org        *hookcamp.Organisation
		err        error
	}{
		{
			name:       "token not provided",
			method:     http.MethodGet,
			statusCode: http.StatusUnauthorized,
			err:        errors.New("please provide a valid token"),
		},
		{
			name:       "invalid token provided",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			token:      "djfbfjhuegj",
			err:        errors.New("please provide a valid token"),
		},
		{
			name:       "valid token",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			token:      org.ApiKey,
			org:        org,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			orgRepo := mocks.NewMockOrganisationRepository(ctrl)

			var times int = 0
			if strings.Contains(tc.name, "valid") {
				times = 1
			}

			orgRepo.EXPECT().
				FetchOrganisationByAPIKey(gomock.Any(), hookcamp.Token(tc.token)).
				Times(times).
				Return(tc.org, tc.err)

			fn := requireAuth(orgRepo)(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				rw.WriteHeader(http.StatusOK)
				rw.Write([]byte(`Hello`))
			}))

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/", nil)

			if tc.token != "" {
				request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", tc.token))
			}

			fn.ServeHTTP(recorder, request)

			if recorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, recorder.Code)
			}
		})
	}

}
