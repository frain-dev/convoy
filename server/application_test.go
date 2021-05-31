package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/golang/mock/gomock"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/mocks"
	goldie "github.com/sebdah/goldie/v2"
)

func verifyMatch(t *testing.T, w httptest.ResponseRecorder) {
	g := goldie.New(t, goldie.WithFixtureDir("./testdata"))
	g.Assert(t, t.Name(), w.Body.Bytes())
}

func TestApplicationHandler_GetApp(t *testing.T) {

	var app *applicationHandler

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := mocks.NewMockOrganisationRepository(ctrl)
	apprepo := mocks.NewMockApplicationRepository(ctrl)

	apprepo.EXPECT().
		FindApplicationByID(gomock.Any(), gomock.Eq("12345")).
		Return(nil, hookcamp.ErrApplicationNotFound)

	validID := "123456789"
	orgID := "1234567890"

	apprepo.EXPECT().
		FindApplicationByID(gomock.Any(), validID).Times(1).
		Return(&hookcamp.Application{
			UID:       validID,
			OrgID:     orgID,
			Title:     "Valid application",
			Endpoints: []hookcamp.Endpoint{},
		}, nil)

	app = newApplicationHandler(apprepo, org)

	tt := []struct {
		name       string
		method     string
		statusCode int
		input      string
	}{
		{
			name:       "app not found",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			input:      "12345",
		},
		{
			name:       "valid application",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			input:      validID,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/apps/%s", tc.input), nil)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tc.input)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

			app.GetApp(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}

			verifyMatch(t, *responseRecorder)
		})
	}

}
