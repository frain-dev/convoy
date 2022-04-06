package server

import (
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/frain-dev/convoy/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Events API", func() {

	// Load Configuration before each test.
	BeforeEach(func() {
		err := config.LoadConfig("testdata/Auth_Config/full-convoy.json")
		Expect(err).ShouldNot(HaveOccurred())
	})

	Context("With application", func() {
		Context("With multiple endpoints", func() {
			It("Creates an event", func() {
				// Arrange.
				var router http.Handler
				var app *applicationHandler

				app = buildApplication()
				router = buildRoutes(app)

				// Act.
				url := "/api/v1/events"
				body := strings.NewReader(`body`)
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

		Context("With a single endpoint", func() {
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
