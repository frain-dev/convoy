package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
)

// CreateFilter
//
//	@Summary		Create a new filter
//	@Description	This endpoint creates a new filter for a subscription
//	@Id				CreateFilter
//	@Tags			Filters
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string						true	"Project ID"
//	@Param			subscriptionID	path		string						true	"Subscription ID"
//	@Param			filter			body		models.CreateFilterRequest	true	"Filter to create"
//	@Success		201				{object}	util.ServerResponse{data=models.FilterResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID}/filters [post]
func (h *Handler) CreateFilter(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subscriptionID := chi.URLParam(r, "subscriptionID")

	var newFilter models.CreateFilterRequest
	if err := util.ReadJSON(r, &newFilter); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// Validate the request
	err = util.Validate(newFilter)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)
	eventTypeRepo := postgres.NewEventTypesRepo(h.A.DB)

	// Check if subscription exists
	_, err = subRepo.FindSubscriptionByID(r.Context(), project.UID, subscriptionID)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("subscription not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
		return
	}

	// check if the event type exists in the project
	exists, err := eventTypeRepo.CheckEventTypeExists(r.Context(), newFilter.EventType, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	if !exists {
		_ = render.Render(w, r, util.NewErrorResponse("event type does not exist", http.StatusNotFound))
		return
	}

	// Check if a filter with the same event type already exists
	existingFilter, err := filterRepo.FindFilterBySubscriptionAndEventType(r.Context(), subscriptionID, newFilter.EventType)
	if err != nil && err.Error() != datastore.ErrFilterNotFound.Error() {
		_ = render.Render(w, r, util.NewErrorResponse("failed to check for existing filter", http.StatusBadRequest))
		return
	}

	if existingFilter != nil {
		_ = render.Render(w, r, util.NewErrorResponse("filter with this event type already exists", http.StatusBadRequest))
		return
	}

	// Create the filter
	filter := &datastore.EventTypeFilter{
		UID:            ulid.Make().String(),
		SubscriptionID: subscriptionID,
		EventType:      newFilter.EventType,
		Headers:        newFilter.Headers,
		Body:           newFilter.Body,
		RawHeaders:     newFilter.Headers,
		RawBody:        newFilter.Body,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	err = filterRepo.CreateFilter(r.Context(), filter)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to create filter", http.StatusBadRequest))
		return
	}

	resp := models.FilterResponse{EventTypeFilter: filter}

	_ = render.Render(w, r, util.NewServerResponse("Filter created successfully", resp, http.StatusCreated))
}

// GetFilter
//
//	@Summary		Get a filter
//	@Description	This endpoint retrieves a single filter
//	@Id				GetFilter
//	@Tags			Filters
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			subscriptionID	path		string	true	"Subscription ID"
//	@Param			filterID		path		string	true	"Filter ID"
//	@Success		200				{object}	util.ServerResponse{data=models.FilterResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID}/filters/{filterID} [get]
func (h *Handler) GetFilter(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subscriptionID := chi.URLParam(r, "subscriptionID")
	filterID := chi.URLParam(r, "filterID")

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)

	// Check if subscription exists
	_, err = subRepo.FindSubscriptionByID(r.Context(), project.UID, subscriptionID)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("subscription not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
		return
	}

	// Get the filter
	filter, err := filterRepo.FindFilterByID(r.Context(), filterID)
	if err != nil {
		if errors.Is(err, datastore.ErrFilterNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("filter not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find filter", http.StatusNotFound))
		return
	}

	// Check if the filter belongs to the subscription
	if filter.SubscriptionID != subscriptionID {
		_ = render.Render(w, r, util.NewErrorResponse("filter does not belong to this subscription", http.StatusNotFound))
		return
	}

	resp := models.FilterResponse{EventTypeFilter: filter}
	_ = render.Render(w, r, util.NewServerResponse("Filter retrieved successfully", resp, http.StatusOK))
}

// GetFilters
//
//	@Summary		List all filters
//	@Description	This endpoint fetches all filters for a subscription
//	@Id				GetFilters
//	@Tags			Filters
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			subscriptionID	path		string	true	"Subscription ID"
//	@Success		200				{object}	util.ServerResponse{data=[]models.FilterResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID}/filters [get]
func (h *Handler) GetFilters(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subscriptionID := chi.URLParam(r, "subscriptionID")

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)

	// Check if subscription exists
	_, err = subRepo.FindSubscriptionByID(r.Context(), project.UID, subscriptionID)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("subscription not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusBadRequest))
		return
	}

	// Get all filters for the subscription
	filters, err := filterRepo.FindFiltersBySubscriptionID(r.Context(), subscriptionID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to find filters", http.StatusNotFound))
		return
	}

	var eventTypeFilters []datastore.EventTypeFilter
	for _, filter := range filters {
		eventTypeFilters = append(eventTypeFilters, datastore.EventTypeFilter{
			UID:            filter.UID,
			SubscriptionID: filter.SubscriptionID,
			EventType:      filter.EventType,
			Headers:        filter.Headers,
			Body:           filter.Body,
			CreatedAt:      filter.CreatedAt,
			UpdatedAt:      filter.UpdatedAt,
		})
	}

	resp := models.NewListResponse(eventTypeFilters, func(f datastore.EventTypeFilter) models.FilterResponse {
		return models.FilterResponse{EventTypeFilter: &f}
	})

	_ = render.Render(w, r, util.NewServerResponse("Filters retrieved successfully", resp, http.StatusOK))
}

// UpdateFilter
//
//	@Summary		Update a filter
//	@Description	This endpoint updates an existing filter
//	@Id				UpdateFilter
//	@Tags			Filters
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string						true	"Project ID"
//	@Param			subscriptionID	path		string						true	"Subscription ID"
//	@Param			filterID		path		string						true	"Filter ID"
//	@Param			filter			body		models.UpdateFilterRequest	true	"Updated filter"
//	@Success		200				{object}	util.ServerResponse{data=models.FilterResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID}/filters/{filterID} [put]
func (h *Handler) UpdateFilter(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subscriptionID := chi.URLParam(r, "subscriptionID")
	filterID := chi.URLParam(r, "filterID")

	var updateFilter models.UpdateFilterRequest
	if err := util.ReadJSON(r, &updateFilter); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// Validate the request
	err = util.Validate(updateFilter)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	eventTypeRepo := postgres.NewEventTypesRepo(h.A.DB)
	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)

	// Check if subscription exists
	_, err = subRepo.FindSubscriptionByID(r.Context(), project.UID, subscriptionID)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("subscription not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
		return
	}

	// check if the event-type exists in the project
	exists, err := eventTypeRepo.CheckEventTypeExists(r.Context(), updateFilter.EventType, project.UID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
		return
	}

	if !exists {
		_ = render.Render(w, r, util.NewErrorResponse("event type does not exist", http.StatusNotFound))
		return
	}

	// Get the filter
	filter, err := filterRepo.FindFilterByID(r.Context(), filterID)
	if err != nil {
		if errors.Is(err, datastore.ErrFilterNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("filter not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find filter", http.StatusNotFound))
		return
	}

	// Check if filter belongs to the subscription
	if filter.SubscriptionID != subscriptionID {
		_ = render.Render(w, r, util.NewErrorResponse("filter does not belong to this subscription", http.StatusNotFound))
		return
	}

	// If event-type is being changed, check if a filter with the new event type already exists
	if updateFilter.EventType != "" && updateFilter.EventType != filter.EventType {
		existingFilter, innerErr := filterRepo.FindFilterBySubscriptionAndEventType(r.Context(), subscriptionID, updateFilter.EventType)
		if innerErr != nil && !errors.Is(innerErr, datastore.ErrFilterNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("failed to check for existing filter", http.StatusBadRequest))
			return
		}

		if existingFilter != nil {
			_ = render.Render(w, r, util.NewErrorResponse("filter with this event type already exists", http.StatusBadRequest))
			return
		}

		filter.EventType = updateFilter.EventType
	}

	// Update the filter
	if updateFilter.Headers != nil {
		filter.Headers = updateFilter.Headers
		filter.RawHeaders = updateFilter.Headers
	}

	if updateFilter.Body != nil {
		filter.Body = updateFilter.Body
		filter.RawBody = updateFilter.Body
	}

	err = filterRepo.UpdateFilter(r.Context(), filter)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to update filter", http.StatusBadRequest))
		return
	}

	resp := models.FilterResponse{EventTypeFilter: filter}
	_ = render.Render(w, r, util.NewServerResponse("Filter updated successfully", resp, http.StatusOK))
}

// DeleteFilter
//
//	@Summary		Delete a filter
//	@Description	This endpoint deletes a filter
//	@Id				DeleteFilter
//	@Tags			Filters
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string	true	"Project ID"
//	@Param			subscriptionID	path		string	true	"Subscription ID"
//	@Param			filterID		path		string	true	"Filter ID"
//	@Success		200				{object}	util.ServerResponse{data=Stub}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID}/filters/{filterID} [delete]
func (h *Handler) DeleteFilter(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subscriptionID := chi.URLParam(r, "subscriptionID")
	filterID := chi.URLParam(r, "filterID")

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)

	// Check if subscription exists
	_, err = subRepo.FindSubscriptionByID(r.Context(), project.UID, subscriptionID)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("subscription not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
		return
	}

	// Get the filter
	filter, err := filterRepo.FindFilterByID(r.Context(), filterID)
	if err != nil {
		if errors.Is(err, datastore.ErrFilterNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("filter not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find filter", http.StatusNotFound))
		return
	}

	// Check if the filter belongs to the subscription
	if filter.SubscriptionID != subscriptionID {
		_ = render.Render(w, r, util.NewErrorResponse("filter does not belong to this subscription", http.StatusNotFound))
		return
	}

	// Delete the filter
	err = filterRepo.DeleteFilter(r.Context(), filterID)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to delete filter", http.StatusBadRequest))
		return
	}

	_ = render.Render(w, r, util.NewServerResponse("Filter deleted successfully", nil, http.StatusOK))
}

// TestFilter
//
//	@Summary		Test a filter
//	@Description	This endpoint tests a filter against a payload
//	@Id				TestFilter
//	@Tags			Filters
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string						true	"Project ID"
//	@Param			subscriptionID	path		string						true	"Subscription ID"
//	@Param			eventType		path		string						true	"Event Type"
//	@Param			payload			body		models.TestFilterRequest	true	"Payload to test"
//	@Success		200				{object}	util.ServerResponse{data=models.TestFilterResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID}/filters/test/{eventType} [post]
func (h *Handler) TestFilter(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subscriptionID := chi.URLParam(r, "subscriptionID")
	eventType := chi.URLParam(r, "eventType")

	var testPayload models.TestFilterRequest
	if err := util.ReadJSON(r, &testPayload); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)

	// Check if subscription exists
	_, err = subRepo.FindSubscriptionByID(r.Context(), project.UID, subscriptionID)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("subscription not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
		return
	}

	// Test the filter
	isMatch, err := filterRepo.TestFilter(r.Context(), subscriptionID, eventType, testPayload.Payload)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to test filter", http.StatusBadRequest))
		return
	}

	resp := models.TestFilterResponse{IsMatch: isMatch}

	_ = render.Render(w, r, util.NewServerResponse("Filter test completed", resp, http.StatusOK))
}

// BulkCreateFilters
//
//	@Summary		Create multiple subscription filters
//	@Description	This endpoint creates multiple filters for a subscription
//	@Id				BulkCreateFilters
//	@Tags			Filters
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string							true	"Project ID"
//	@Param			subscriptionID	path		string							true	"Subscription ID"
//	@Param			filters			body		[]models.CreateFilterRequest	true	"Filters to create"
//	@Success		201				{object}	util.ServerResponse{data=[]models.FilterResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID}/filters/bulk [post]
func (h *Handler) BulkCreateFilters(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subscriptionID := chi.URLParam(r, "subscriptionID")

	var newFilters []models.CreateFilterRequest
	if err := util.ReadJSON(r, &newFilters); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// Validate the request
	for _, filter := range newFilters {
		err := util.Validate(filter)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("Invalid filter for event type %s: %s", filter.EventType, err.Error()), http.StatusBadRequest))
			return
		}
	}

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)
	eventTypeRepo := postgres.NewEventTypesRepo(h.A.DB)

	// Check if subscription exists
	_, err = subRepo.FindSubscriptionByID(r.Context(), project.UID, subscriptionID)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("subscription not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
		return
	}

	// Prepare filters for creation
	filtersToCreate := make([]datastore.EventTypeFilter, 0, len(newFilters))
	eventTypeMap := make(map[string]bool)

	// First check if all event types exist
	for _, filter := range newFilters {
		// Check if event type exists
		exists, err2 := eventTypeRepo.CheckEventTypeExists(r.Context(), filter.EventType, project.UID)
		if err2 != nil {
			_ = render.Render(w, r, util.NewErrorResponse(err2.Error(), http.StatusNotFound))
			return
		}

		if !exists {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("event type %s does not exist", filter.EventType), http.StatusNotFound))
			return
		}

		// Check for duplicate event types in the request
		if _, exists = eventTypeMap[filter.EventType]; exists {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("duplicate event type %s in request", filter.EventType), http.StatusBadRequest))
			return
		}
		eventTypeMap[filter.EventType] = true
	}

	// Get existing filters for this subscription
	existingFilters, err := filterRepo.FindFiltersBySubscriptionID(r.Context(), subscriptionID)
	if err != nil && !errors.Is(err, datastore.ErrFilterNotFound) {
		_ = render.Render(w, r, util.NewErrorResponse("failed to check for existing filters", http.StatusInternalServerError))
		return
	}

	// Check for conflicts with existing filters
	for _, existingFilter := range existingFilters {
		if _, exists := eventTypeMap[existingFilter.EventType]; exists {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("filter for event type %s already exists", existingFilter.EventType), http.StatusBadRequest))
			return
		}
	}

	// Build filters to create
	for _, filter := range newFilters {
		filtersToCreate = append(filtersToCreate, datastore.EventTypeFilter{
			UID:            ulid.Make().String(),
			SubscriptionID: subscriptionID,
			EventType:      filter.EventType,
			Headers:        filter.Headers,
			Body:           filter.Body,
			RawHeaders:     filter.Headers,
			RawBody:        filter.Body,
		})
	}

	// Create filters in a transaction
	err = filterRepo.CreateFilters(r.Context(), filtersToCreate)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse("failed to create filters", http.StatusBadRequest))
		return
	}

	// Prepare response
	responseFilters := make([]models.FilterResponse, 0, len(filtersToCreate))
	for _, filter := range filtersToCreate {
		responseFilters = append(responseFilters, models.FilterResponse{EventTypeFilter: &filter})
	}

	_ = render.Render(w, r, util.NewServerResponse("Filters created successfully", responseFilters, http.StatusCreated))
}

// BulkUpdateFilters
//
//	@Summary		Update multiple subscription filters
//	@Description	This endpoint updates multiple filters for a subscription
//	@Id				BulkUpdateFilters
//	@Tags			Filters
//	@Accept			json
//	@Produce		json
//	@Param			projectID		path		string								true	"Project ID"
//	@Param			subscriptionID	path		string								true	"Subscription ID"
//	@Param			filters			body		[]models.BulkUpdateFilterRequest	true	"Filters to update"
//	@Success		200				{object}	util.ServerResponse{data=[]models.FilterResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID}/filters/bulk_update [put]
func (h *Handler) BulkUpdateFilters(w http.ResponseWriter, r *http.Request) {
	project, err := h.retrieveProject(r)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subscriptionID := chi.URLParam(r, "subscriptionID")

	var updateFilters []models.BulkUpdateFilterRequest
	if err := util.ReadJSON(r, &updateFilters); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// Validate the request
	for _, filter := range updateFilters {
		err := util.Validate(filter)
		if err != nil {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("Invalid filter %s: %s", filter.UID, err.Error()), http.StatusBadRequest))
			return
		}
	}

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)
	eventTypeRepo := postgres.NewEventTypesRepo(h.A.DB)

	// Check if subscription exists
	_, err = subRepo.FindSubscriptionByID(r.Context(), project.UID, subscriptionID)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("subscription not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
		return
	}

	// Get existing filters for this subscription
	existingFilters, err := filterRepo.FindFiltersBySubscriptionID(r.Context(), subscriptionID)
	if err != nil && !errors.Is(err, datastore.ErrFilterNotFound) {
		_ = render.Render(w, r, util.NewErrorResponse("failed to check for existing filters", http.StatusBadRequest))
		return
	}

	// Map to track existing filters by ID
	existingFiltersMap := make(map[string]datastore.EventTypeFilter)
	existingEventTypesMap := make(map[string]string) // eventType -> filterID

	for _, filter := range existingFilters {
		existingFiltersMap[filter.UID] = filter
		existingEventTypesMap[filter.EventType] = filter.UID
	}

	// Track event types for duplicate checking
	eventTypeMap := make(map[string]string)

	// Validate each filter update
	for _, filterUpdate := range updateFilters {
		// Check if filter exists and belongs to this subscription
		existingFilter, exists := existingFiltersMap[filterUpdate.UID]
		if !exists {
			_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("filter %s not found", filterUpdate.UID), http.StatusNotFound))
			return
		}

		// If event type is being changed
		if filterUpdate.EventType != "" && filterUpdate.EventType != existingFilter.EventType {
			// Check if the new event type exists
			exists, err = eventTypeRepo.CheckEventTypeExists(r.Context(), filterUpdate.EventType, project.UID)
			if err != nil {
				_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusNotFound))
				return
			}

			if !exists {
				_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("event type %s does not exist", filterUpdate.EventType), http.StatusNotFound))
				return
			}

			// Check if another filter already uses this event type (excluding the current filter being updated)
			if existingID, e := existingEventTypesMap[filterUpdate.EventType]; e && existingID != filterUpdate.UID {
				_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("filter for event type %s already exists", filterUpdate.EventType), http.StatusBadRequest))
				return
			}

			// Check for duplicates within the update request
			if previousUID, e := eventTypeMap[filterUpdate.EventType]; e && previousUID != filterUpdate.UID {
				_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("duplicate event type %s in request", filterUpdate.EventType), http.StatusBadRequest))
				return
			}
			eventTypeMap[filterUpdate.EventType] = filterUpdate.UID
		} else if filterUpdate.EventType != "" {
			// If not changing event type, just make sure we're tracking it for this UID
			eventTypeMap[filterUpdate.EventType] = filterUpdate.UID
		} else {
			// If event type not specified, use existing one and track it for this UID
			eventTypeMap[existingFilter.EventType] = filterUpdate.UID
		}
	}

	// Process updates
	updatedFilters := make([]datastore.EventTypeFilter, 0, len(updateFilters))

	for _, filterUpdate := range updateFilters {
		existingFilter := existingFiltersMap[filterUpdate.UID]

		// Apply updates
		if filterUpdate.EventType != "" {
			existingFilter.EventType = filterUpdate.EventType
		}

		if filterUpdate.Headers != nil {
			existingFilter.Headers = filterUpdate.Headers
			existingFilter.RawHeaders = filterUpdate.Headers
		}

		if filterUpdate.Body != nil {
			existingFilter.Body = filterUpdate.Body
			existingFilter.RawBody = filterUpdate.Body
		}

		existingFilter.UpdatedAt = time.Now()
		updatedFilters = append(updatedFilters, existingFilter)
	}

	// Update all filters in a single transaction
	err = filterRepo.UpdateFilters(r.Context(), updatedFilters)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(fmt.Sprintf("failed to update filters: %s", err.Error()), http.StatusBadRequest))
		return
	}

	// Prepare response
	responseFilters := make([]models.FilterResponse, 0, len(updatedFilters))
	for _, filter := range updatedFilters {
		responseFilters = append(responseFilters, models.FilterResponse{EventTypeFilter: &filter})
	}

	_ = render.Render(w, r, util.NewServerResponse("Filters updated successfully", responseFilters, http.StatusOK))
}
