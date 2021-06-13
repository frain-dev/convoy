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

	orgID := "1234567890"

	organisation := &hookcamp.Organisation{
		UID: orgID,
	}

	validID := "123456789"

	// apprepo.EXPECT().

	app = newApplicationHandler(apprepo, org)

	tt := []struct {
		name       string
		method     string
		statusCode int
		input      string
		dbFn       func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository)
	}{
		{
			name:       "app not found",
			method:     http.MethodGet,
			statusCode: http.StatusNotFound,
			input:      "12345",
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).
					Return(nil, hookcamp.ErrApplicationNotFound).Times(1)
			},
		},
		{
			name:       "valid application",
			method:     http.MethodGet,
			statusCode: http.StatusOK,
			input:      validID,
			dbFn: func(appRepo *mocks.MockApplicationRepository, orgRepo *mocks.MockOrganisationRepository) {
				appRepo.EXPECT().
					FindApplicationByID(gomock.Any(), gomock.Any()).Times(1).
					Return(&hookcamp.Application{
						UID:       validID,
						OrgID:     orgID,
						Title:     "Valid application",
						Endpoints: []hookcamp.Endpoint{},
					}, nil)

			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, fmt.Sprintf("/v1/apps/%s", tc.input), nil)
			responseRecorder := httptest.NewRecorder()
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tc.input)

			request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

			request = request.WithContext(setOrgInContext(request.Context(), organisation))

			if tc.dbFn != nil {
				tc.dbFn(apprepo, org)
			}

			requireAppOwnership(apprepo)(http.HandlerFunc(app.GetApp)).
				ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != tc.statusCode {
				t.Errorf("Want status '%d', got '%d'", tc.statusCode, responseRecorder.Code)
			}

			verifyMatch(t, *responseRecorder)
		})
	}

}
