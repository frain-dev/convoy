package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/server/testdb"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Events API", func() {
	var db datastore.DatabaseClient
	var router http.Handler
	var convoyApp *applicationHandler

	url := "/api/v1/events"

	// Load Configuration before each test.
	BeforeEach(func() {
		convoyApp = buildApplication()
		router = buildRoutes(convoyApp)

		err := config.LoadConfig("testdata/Auth_Config/full-convoy.json")
		Expect(err).ShouldNot(HaveOccurred())

		db = getDB()
		testdb.PurgeDB(db)
	})

	AfterEach(func() {
		// purgeDB
		//db = getDB()
		//testdb.PurgeDB(db)
	})

	Context("With application", func() {
		Context("With no endpoint", func() {})
		Context("With a single endpoint", func() {
			var app *datastore.Application
			var group *datastore.Group

			BeforeEach(func() {
				// Arrange - Data
				group, _ = testdb.SeedDefaultGroup(db)
				app, _ = testdb.SeedApplication(db, group, "", false)
				_, _ = testdb.SeedEndpoint(db, app, 1)
			})

			It("Creates an event", func() {
				// Act.
				plainBody := fmt.Sprintf(`
					{
						"app_id": "%s",
						"event_type": "payment.created",
						"data": {
							"event": "payment.created",
							"data": {}
						}
					}
				`, app.UID)

				body := strings.NewReader(plainBody)
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, url, body)
				req.SetBasicAuth("test", "test")
				req.Header.Add("Content-Type", "application/json")

				router.ServeHTTP(w, req)
				resp := w.Result()

				// Assert.
				// TODO(subomi): Verify payload.
				Expect(resp).To(HaveHTTPStatus(http.StatusCreated))

				By("Should create one endpoint")
			})
		})

		Context("With multiple endpoints", func() {
			var app *datastore.Application
			var group *datastore.Group

			BeforeEach(func() {
				// Arrange - Data.
				group, _ = testdb.SeedDefaultGroup(db)
				app, _ = testdb.SeedApplication(db, group, "", false)
				_, _ = testdb.SeedEndpoint(db, app, 2)
			})

			It("Creates an event", func() {
				// Act.
				plainBody := fmt.Sprintf(`
					{
						"app_id": "%s",
						"event_type": "payment.created",
						"data": {
							"event": "payment.created",
							"data": {}
						}
					}
				`, app.UID)

				body := strings.NewReader(plainBody)
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, url, body)
				req.SetBasicAuth("test", "test")
				req.Header.Add("Content-Type", "application/json")

				router.ServeHTTP(w, req)
				resp := w.Result()

				// Assert
				Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				By("Should create two event deliveries")
				respBody, err := ioutil.ReadAll(resp.Body)
				Expect(err).ShouldNot(HaveOccurred())

				var sR serverResponse
				err = json.Unmarshal(respBody, &sR)

				var event datastore.Event
				err = json.Unmarshal(sR.Data, &event)
				Expect(err).ShouldNot(HaveOccurred())

				eventDeliveryRepo := db.EventDeliveryRepo()
				results, err := eventDeliveryRepo.FindEventDeliveriesByEventID(context.Background(), event.UID)
				fmt.Fprintf(GinkgoWriter, "%+v", results)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(len(results)).Should(Equal(2))
			})
		})
	})

	Context("With a disabled application", func() {
		var app *datastore.Application
		var group *datastore.Group

		BeforeEach(func() {
			// Arrange - Data
			group, _ = testdb.SeedDefaultGroup(db)
			app, _ = testdb.SeedApplication(db, group, "", true)
			_, _ = testdb.SeedEndpoint(db, app, 1)
		})

		It("Creates an event", func() {
			// Act.
			plainBody := fmt.Sprintf(`
				{
					"app_id": "%s",
					"event_type": "payment.created",
					"data": {
						"event": "payment.created",
						"data": {}
					}
				}
			`, app.UID)

			body := strings.NewReader(plainBody)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, url, body)
			req.SetBasicAuth("test", "test")
			req.Header.Add("Content-Type", "application/json")

			router.ServeHTTP(w, req)
			resp := w.Result()

			// Assert
			Expect(resp).To(HaveHTTPStatus(http.StatusCreated))
		})
		It("Should nor create an event delivery", func() {})
	})

	Context("With bad application", func() {
	})
})
