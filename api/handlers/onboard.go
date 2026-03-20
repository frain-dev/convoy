package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/frain-dev/convoy/worker/task"
)

const maxOnboardItems = 1000
const onboardBatchSize = 50

// BulkOnboard
//
//	@Summary		Bulk onboard endpoints with subscriptions
//	@Description	This endpoint accepts a CSV file or JSON body to bulk-create endpoints with subscriptions
//	@Tags			Onboard
//	@Id				BulkOnboard
//	@Accept			json,mpfd
//	@Produce		json
//	@Param			projectID	path		string						true	"Project ID"
//	@Param			dry_run		query		bool						false	"Validate without creating"
//	@Param			onboard		body		models.BulkOnboardRequest	false	"Onboard Details (JSON)"
//	@Param			file		formData	file						false	"CSV file upload"
//	@Success		202			{object}	util.ServerResponse{data=models.BulkOnboardAcceptedResponse}
//	@Success		200			{object}	util.ServerResponse{data=models.BulkOnboardDryRunResponse}
//	@Failure		400,401,404	{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/onboard [post]
func (h *Handler) BulkOnboard(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("Project not found", http.StatusBadRequest))
		return
	}

	if project.Type == datastore.IncomingProject {
		_ = render.Render(w, r, util.NewErrorResponse("bulk onboard is only supported for outgoing projects", http.StatusBadRequest))
		return
	}

	isDryRun := r.URL.Query().Get("dry_run") == "true"

	var items []models.OnboardItem

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		items, err = parseCSVUpload(r)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
			return
		}
	} else {
		var req models.BulkOnboardRequest
		err = util.ReadJSON(r, &req)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse("Invalid request format", http.StatusBadRequest))
			return
		}
		items = req.Items
	}

	if len(items) == 0 {
		_ = render.Render(w, r, util.NewErrorResponse("items list cannot be empty", http.StatusBadRequest))
		return
	}

	if len(items) > maxOnboardItems {
		_ = render.Render(w, r, util.NewErrorResponse(
			fmt.Sprintf("items list exceeds maximum size of %d", maxOnboardItems), http.StatusBadRequest))
		return
	}

	// Validate all items
	var validationErrors []models.OnboardValidationError
	for i, item := range items {
		row := i + 1

		if strings.TrimSpace(item.Name) == "" {
			validationErrors = append(validationErrors, models.OnboardValidationError{
				Row: row, Field: "name", Message: "name is required",
			})
		}

		_, urlErr := services.ValidateEndpointURL(item.URL, project.Config.SSL.EnforceSecureEndpoints)
		if urlErr != nil {
			validationErrors = append(validationErrors, models.OnboardValidationError{
				Row: row, Field: "url", Message: urlErr.Error(),
			})
		}

		if item.EventType == "" {
			items[i].EventType = "*"
		}

		hasUsername := strings.TrimSpace(item.AuthUsername) != ""
		hasPassword := strings.TrimSpace(item.AuthPassword) != ""
		if hasUsername != hasPassword {
			validationErrors = append(validationErrors, models.OnboardValidationError{
				Row: row, Field: "auth_username/auth_password", Message: "both auth_username and auth_password must be provided together",
			})
		}
	}

	if isDryRun {
		resp := models.BulkOnboardDryRunResponse{
			TotalRows:  len(items),
			ValidCount: len(items) - countUniqueRows(validationErrors),
			Errors:     validationErrors,
		}
		if resp.Errors == nil {
			resp.Errors = []models.OnboardValidationError{}
		}
		_ = render.Render(w, r, util.NewServerResponse("Dry run validation complete", resp, http.StatusOK))
		return
	}

	if len(validationErrors) > 0 {
		_ = render.Render(w, r, util.NewErrorResponseWithData("Validation failed", http.StatusBadRequest, models.BulkOnboardDryRunResponse{
			TotalRows:  len(items),
			ValidCount: len(items) - countUniqueRows(validationErrors),
			Errors:     validationErrors,
		}))
		return
	}

	// Phase 1: Build all batch payloads (fail fast on encoding errors)
	type batchJob struct {
		job *queue.Job
	}
	var jobs []batchJob
	for i := 0; i < len(items); i += onboardBatchSize {
		end := min(i+onboardBatchSize, len(items))

		batchID := ulid.Make().String()
		batch := task.BulkOnboardBatch{
			ProjectID: project.UID,
			BatchID:   batchID,
			Items:     toBatchItems(items[i:end]),
		}

		payload, encErr := msgpack.EncodeMsgPack(batch)
		if encErr != nil {
			_ = render.Render(w, r, util.NewErrorResponse("Failed to encode batch payload", http.StatusInternalServerError))
			return
		}

		jobs = append(jobs, batchJob{
			job: &queue.Job{
				ID:      queue.JobId{ProjectID: project.UID, ResourceID: batchID}.OnboardJobId(),
				Payload: payload,
			},
		})
	}

	// Phase 2: Enqueue all jobs
	for _, j := range jobs {
		writeErr := h.A.Queue.Write(convoy.BulkOnboardProcessor, convoy.DefaultQueue, j.job)
		if writeErr != nil {
			_ = render.Render(w, r, util.NewErrorResponse("Failed to queue batch", http.StatusInternalServerError))
			return
		}
	}
	batchCount := len(jobs)

	resp := models.BulkOnboardAcceptedResponse{
		BatchCount: batchCount,
		TotalItems: len(items),
		Message:    "Bulk onboard request accepted for processing",
	}
	_ = render.Render(w, r, util.NewServerResponse("Bulk onboard request accepted", resp, http.StatusAccepted))
}

func parseCSVUpload(r *http.Request) ([]models.OnboardItem, error) {
	err := r.ParseMultipartForm(10 << 20) // 10 MB
	if err != nil {
		log.WithError(err).Error("bulk onboard: failed to parse multipart form")
		return nil, fmt.Errorf("failed to parse multipart form")
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		log.WithError(err).Error("bulk onboard: failed to read file field")
		return nil, fmt.Errorf("failed to read file field")
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read and validate header
	header, err := reader.Read()
	if err != nil {
		log.WithError(err).Error("bulk onboard: failed to read CSV header")
		return nil, fmt.Errorf("failed to read CSV header")
	}

	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.TrimSpace(strings.ToLower(col))] = i
	}

	requiredCols := []string{"name", "url"}
	for _, col := range requiredCols {
		if _, ok := colMap[col]; !ok {
			return nil, fmt.Errorf("CSV missing required column: %s", col)
		}
	}

	expectedCols := len(header)
	var items []models.OnboardItem
	rowNum := 1
	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		rowNum++
		if readErr != nil {
			log.WithError(readErr).Errorf("bulk onboard: failed to parse CSV row %d", rowNum)
			return nil, fmt.Errorf("CSV row %d: failed to parse row", rowNum)
		}

		if len(record) != expectedCols {
			return nil, fmt.Errorf("CSV row %d: expected %d columns but got %d", rowNum, expectedCols, len(record))
		}

		item := models.OnboardItem{
			Name: getCSVField(record, colMap, "name"),
			URL:  getCSVField(record, colMap, "url"),
		}

		if idx, ok := colMap["event_type"]; ok && idx < len(record) {
			item.EventType = strings.TrimSpace(record[idx])
		}
		if idx, ok := colMap["auth_username"]; ok && idx < len(record) {
			item.AuthUsername = strings.TrimSpace(record[idx])
		}
		if idx, ok := colMap["auth_password"]; ok && idx < len(record) {
			item.AuthPassword = strings.TrimSpace(record[idx])
		}

		items = append(items, item)

		if len(items) > maxOnboardItems {
			return nil, fmt.Errorf("CSV file exceeds maximum of %d items", maxOnboardItems)
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("CSV file is empty or contains only a header row")
	}

	return items, nil
}

func getCSVField(record []string, colMap map[string]int, col string) string {
	if idx, ok := colMap[col]; ok && idx < len(record) {
		return strings.TrimSpace(record[idx])
	}
	return ""
}

func toBatchItems(items []models.OnboardItem) []task.BulkOnboardItem {
	result := make([]task.BulkOnboardItem, len(items))
	for i, item := range items {
		result[i] = task.BulkOnboardItem{
			Name:         item.Name,
			URL:          item.URL,
			EventType:    item.EventType,
			AuthUsername: item.AuthUsername,
			AuthPassword: item.AuthPassword,
		}
	}
	return result
}

func countUniqueRows(errors []models.OnboardValidationError) int {
	seen := make(map[int]bool)
	for _, e := range errors {
		seen[e.Row] = true
	}
	return len(seen)
}
