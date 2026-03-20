package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/api/models"
)

// onboardAPIResponse is the generic server response wrapper for the onboard endpoint.
type onboardAPIResponse struct {
	Status  bool            `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// postOnboard sends a JSON onboard request and returns the raw response.
func postOnboard(t *testing.T, serverURL, projectID, apiKey string, payload models.BulkOnboardRequest, dryRun bool) *http.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	require.NoError(t, err)

	url := fmt.Sprintf("%s/api/v1/projects/%s/onboard", serverURL, projectID)
	if dryRun {
		url += "?dry_run=true"
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// postOnboardCSV sends a CSV file onboard request and returns the raw response.
func postOnboardCSV(t *testing.T, serverURL, projectID, apiKey, csvContent string, dryRun bool) *http.Response {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "test.csv")
	require.NoError(t, err)
	_, err = part.Write([]byte(csvContent))
	require.NoError(t, err)
	writer.Close()

	url := fmt.Sprintf("%s/api/v1/projects/%s/onboard", serverURL, projectID)
	if dryRun {
		url += "?dry_run=true"
	}

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// listEndpoints fetches endpoints from the API and returns the count.
func listEndpoints(t *testing.T, serverURL, projectID, apiKey string) int {
	t.Helper()

	url := fmt.Sprintf("%s/api/v1/projects/%s/endpoints?perPage=300", serverURL, projectID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result struct {
		Data struct {
			Content []json.RawMessage `json:"content"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	return len(result.Data.Content)
}

// listSubscriptions fetches subscriptions from the API and returns the count.
func listSubscriptions(t *testing.T, serverURL, projectID, apiKey string) int {
	t.Helper()

	url := fmt.Sprintf("%s/api/v1/projects/%s/subscriptions?perPage=300", serverURL, projectID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var result struct {
		Data struct {
			Content []json.RawMessage `json:"content"`
		} `json:"data"`
	}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	return len(result.Data.Content)
}

func TestE2E_BulkOnboard_JSON_CreatesEndpointsAndSubscriptions(t *testing.T) {
	env := SetupE2E(t)

	items := []models.OnboardItem{
		{Name: "Onboard EP 1", URL: "http://localhost:29001/webhook", EventType: "order.created"},
		{Name: "Onboard EP 2", URL: "http://localhost:29002/webhook"},
		{Name: "Onboard EP 3", URL: "http://localhost:29003/webhook", EventType: "user.signup"},
	}

	resp := postOnboard(t, env.ServerURL, env.Project.UID, env.APIKey,
		models.BulkOnboardRequest{Items: items}, false)
	defer resp.Body.Close()

	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var apiResp onboardAPIResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err)
	require.True(t, apiResp.Status)

	var accepted models.BulkOnboardAcceptedResponse
	err = json.Unmarshal(apiResp.Data, &accepted)
	require.NoError(t, err)
	require.Equal(t, 1, accepted.BatchCount)
	require.Equal(t, 3, accepted.TotalItems)

	// Wait for worker to process the batch
	time.Sleep(5 * time.Second)

	// Verify endpoints and subscriptions were created
	epCount := listEndpoints(t, env.ServerURL, env.Project.UID, env.APIKey)
	require.Equal(t, 3, epCount, "Expected 3 endpoints to be created by worker")

	subCount := listSubscriptions(t, env.ServerURL, env.Project.UID, env.APIKey)
	require.Equal(t, 3, subCount, "Expected 3 subscriptions to be created by worker")
}

func TestE2E_BulkOnboard_CSV_CreatesEndpointsAndSubscriptions(t *testing.T) {
	env := SetupE2E(t)

	csvContent := "name,url,event_type,auth_username,auth_password\n" +
		"CSV EP 1,http://localhost:29011/csv1,payment.received,,\n" +
		"CSV EP 2,http://localhost:29012/csv2,*,,\n" +
		"CSV EP 3,http://localhost:29013/csv3,order.shipped,,\n"

	resp := postOnboardCSV(t, env.ServerURL, env.Project.UID, env.APIKey, csvContent, false)
	defer resp.Body.Close()

	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var apiResp onboardAPIResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err)

	var accepted models.BulkOnboardAcceptedResponse
	err = json.Unmarshal(apiResp.Data, &accepted)
	require.NoError(t, err)
	require.Equal(t, 1, accepted.BatchCount)
	require.Equal(t, 3, accepted.TotalItems)

	time.Sleep(5 * time.Second)

	epCount := listEndpoints(t, env.ServerURL, env.Project.UID, env.APIKey)
	require.Equal(t, 3, epCount)

	subCount := listSubscriptions(t, env.ServerURL, env.Project.UID, env.APIKey)
	require.Equal(t, 3, subCount)
}

func TestE2E_BulkOnboard_DryRun_DoesNotCreateResources(t *testing.T) {
	env := SetupE2E(t)

	items := []models.OnboardItem{
		{Name: "DryRun EP 1", URL: "http://localhost:29021/webhook"},
		{Name: "DryRun EP 2", URL: "http://localhost:29022/webhook"},
	}

	resp := postOnboard(t, env.ServerURL, env.Project.UID, env.APIKey,
		models.BulkOnboardRequest{Items: items}, true)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var apiResp onboardAPIResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err)

	var dryRunResp models.BulkOnboardDryRunResponse
	err = json.Unmarshal(apiResp.Data, &dryRunResp)
	require.NoError(t, err)
	require.Equal(t, 2, dryRunResp.TotalRows)
	require.Equal(t, 2, dryRunResp.ValidCount)
	require.Empty(t, dryRunResp.Errors)

	// Wait a bit, then verify nothing was created
	time.Sleep(3 * time.Second)

	epCount := listEndpoints(t, env.ServerURL, env.Project.UID, env.APIKey)
	require.Equal(t, 0, epCount, "Dry run should not create any endpoints")

	subCount := listSubscriptions(t, env.ServerURL, env.Project.UID, env.APIKey)
	require.Equal(t, 0, subCount, "Dry run should not create any subscriptions")
}

func TestE2E_BulkOnboard_DryRun_ReturnsValidationErrors(t *testing.T) {
	env := SetupE2E(t)

	items := []models.OnboardItem{
		{Name: "", URL: "http://ok.com/hook"},                                // missing name
		{Name: "No URL", URL: ""},                                            // missing url
		{Name: "Bad Scheme", URL: "ftp://bad.com/hook"},                      // bad scheme
		{Name: "Half Auth", URL: "http://ok.com/hook", AuthUsername: "user"}, // incomplete auth
	}

	resp := postOnboard(t, env.ServerURL, env.Project.UID, env.APIKey,
		models.BulkOnboardRequest{Items: items}, true)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var apiResp onboardAPIResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err)

	var dryRunResp models.BulkOnboardDryRunResponse
	err = json.Unmarshal(apiResp.Data, &dryRunResp)
	require.NoError(t, err)
	require.Equal(t, 4, dryRunResp.TotalRows)
	require.Equal(t, 0, dryRunResp.ValidCount)
	require.Len(t, dryRunResp.Errors, 4)

	// Verify error fields
	errFields := make(map[int]string)
	for _, e := range dryRunResp.Errors {
		errFields[e.Row] = e.Field
	}
	require.Equal(t, "name", errFields[1])
	require.Equal(t, "url", errFields[2])
	require.Equal(t, "url", errFields[3])
	require.Equal(t, "auth_username/auth_password", errFields[4])
}

func TestE2E_BulkOnboard_ValidationFailure_Returns400(t *testing.T) {
	env := SetupE2E(t)

	items := []models.OnboardItem{
		{Name: "", URL: "http://ok.com/hook"}, // invalid
	}

	resp := postOnboard(t, env.ServerURL, env.Project.UID, env.APIKey,
		models.BulkOnboardRequest{Items: items}, false)
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var apiResp onboardAPIResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err)
	require.False(t, apiResp.Status, "validation failure response must have status: false")
}

func TestE2E_BulkOnboard_EmptyItems_Returns400(t *testing.T) {
	env := SetupE2E(t)

	resp := postOnboard(t, env.ServerURL, env.Project.UID, env.APIKey,
		models.BulkOnboardRequest{Items: []models.OnboardItem{}}, false)
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var apiResp onboardAPIResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err)
	require.Contains(t, apiResp.Message, "empty")
}

func TestE2E_BulkOnboard_MultipleBatches(t *testing.T) {
	env := SetupE2E(t)

	// 75 items -> 2 batches (50 + 25)
	items := make([]models.OnboardItem, 75)
	for i := range items {
		items[i] = models.OnboardItem{
			Name: fmt.Sprintf("Batch EP %d", i+1),
			URL:  fmt.Sprintf("http://localhost:%d/webhook", 29100+i),
		}
	}

	resp := postOnboard(t, env.ServerURL, env.Project.UID, env.APIKey,
		models.BulkOnboardRequest{Items: items}, false)
	defer resp.Body.Close()

	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var apiResp onboardAPIResponse
	err = json.Unmarshal(body, &apiResp)
	require.NoError(t, err)

	var accepted models.BulkOnboardAcceptedResponse
	err = json.Unmarshal(apiResp.Data, &accepted)
	require.NoError(t, err)
	require.Equal(t, 2, accepted.BatchCount)
	require.Equal(t, 75, accepted.TotalItems)

	// Wait for worker to process both batches
	time.Sleep(10 * time.Second)

	epCount := listEndpoints(t, env.ServerURL, env.Project.UID, env.APIKey)
	require.Equal(t, 75, epCount, "Expected 75 endpoints to be created across 2 batches")

	subCount := listSubscriptions(t, env.ServerURL, env.Project.UID, env.APIKey)
	require.Equal(t, 75, subCount, "Expected 75 subscriptions to be created across 2 batches")
}
