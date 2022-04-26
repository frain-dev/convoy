package server

import (
	"fmt"
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

	// Load Configuration before each test.
	BeforeEach(func() {
		err := config.LoadConfig("testdata/Auth_Config/full-convoy.json")
		Expect(err).ShouldNot(HaveOccurred())

		db = getDB()
	})

	AfterEach(func() {
		// purgeDB
		db = getDB()
		testdb.PurgeDB(db)
	})

	Context("With application", func() {
		Context("With a single endpoint", func() {
			It("Creates an event", func() {
				// Arrange.
				var router http.Handler
				var convoyApp *applicationHandler

				convoyApp = buildApplication()
				router = buildRoutes(convoyApp)

				// Arrange - Data
				group, _ := testdb.SeedDefaultGroup(db)
				app, _ := testdb.SeedApplication(db, group)
				_, _ = testdb.SeedEndpoint(db, app)

				// Act.
				url := "/api/v1/events"
				plainBody := fmt.Sprintf(`
					{
						"app_id": "%s",
						"event_type": "payment.created",
						"data": {
							"event": "payment.created",
							"data": {
								"status": "Completed",
								"description": "Transaction successful"
							}
						}
					}
				`, app.UID)

				fmt.Printf("BODY: %v", plainBody)
				body := strings.NewReader(plainBody)
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, url, body)
				req.SetBasicAuth("test", "test")
				req.Header.Add("Content-Type", "application/json")

				router.ServeHTTP(w, req)

				resp := w.Result()

				// Assert.
				Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			})
			It("Creates multiple event deliveries", func() {})
		})

		Context("With multiple endpoints", func() {
			It("Creates an event", func() {})
			It("Creates an event delivery", func() {})
		})
	})

	Context("With a disabled application", func() {
		It("Creates an event", func() {})
		It("Should nor create an event delivery", func() {})
	})

	Context("Without application", func() {
	})
})
