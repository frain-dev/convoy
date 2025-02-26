package handlers

import (
	"errors"
	"github.com/frain-dev/convoy/database/postgres"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/util"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/oklog/ulid/v2"
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
	projectID := chi.URLParam(r, "projectID")
	subscriptionID := chi.URLParam(r, "subscriptionID")

	var newFilter models.CreateFilterRequest
	if err := util.ReadJSON(r, &newFilter); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// Validate the request
	err := util.Validate(newFilter)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)

	// Check if subscription exists
	_, err = subRepo.FindSubscriptionByID(r.Context(), projectID, subscriptionID)
	if err != nil {
		if errors.Is(err, datastore.ErrSubscriptionNotFound) {
			_ = render.Render(w, r, util.NewErrorResponse("subscription not found", http.StatusNotFound))
			return
		}
		_ = render.Render(w, r, util.NewErrorResponse("failed to find subscription", http.StatusNotFound))
		return
	}

	// Check if filter with same event type already exists
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
	projectID := chi.URLParam(r, "projectID")
	subscriptionID := chi.URLParam(r, "subscriptionID")
	filterID := chi.URLParam(r, "filterID")

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)

	// Check if subscription exists
	_, err := subRepo.FindSubscriptionByID(r.Context(), projectID, subscriptionID)
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

	// Check if filter belongs to the subscription
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
//	@Success		200				{object}	util.ServerResponse{data=models.FiltersResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID}/filters [get]
func (h *Handler) GetFilters(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	subscriptionID := chi.URLParam(r, "subscriptionID")

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)

	// Check if subscription exists
	_, err := subRepo.FindSubscriptionByID(r.Context(), projectID, subscriptionID)
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
	projectID := chi.URLParam(r, "projectID")
	subscriptionID := chi.URLParam(r, "subscriptionID")
	filterID := chi.URLParam(r, "filterID")

	var updateFilter models.UpdateFilterRequest
	if err := util.ReadJSON(r, &updateFilter); err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	// Validate the request
	err := util.Validate(updateFilter)
	if err != nil {
		_ = render.Render(w, r, util.NewErrorResponse(err.Error(), http.StatusBadRequest))
		return
	}

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)

	// Check if subscription exists
	_, err = subRepo.FindSubscriptionByID(r.Context(), projectID, subscriptionID)
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
	projectID := chi.URLParam(r, "projectID")
	subscriptionID := chi.URLParam(r, "subscriptionID")
	filterID := chi.URLParam(r, "filterID")

	subRepo := postgres.NewSubscriptionRepo(h.A.DB)
	filterRepo := postgres.NewFilterRepo(h.A.DB)

	// Check if subscription exists
	_, err := subRepo.FindSubscriptionByID(r.Context(), projectID, subscriptionID)
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

	// Check if filter belongs to the subscription
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
//	@Param			projectID		path		string					true	"Project ID"
//	@Param			subscriptionID	path		string					true	"Subscription ID"
//	@Param			eventType		path		string					true	"Event Type"
//	@Param			payload			body		models.TestFilterRequest	true	"Payload to test"
//	@Success		200				{object}	util.ServerResponse{data=models.TestFilterResponse}
//	@Failure		400,401,404		{object}	util.ServerResponse{data=Stub}
//	@Security		ApiKeyAuth
//	@Router			/v1/projects/{projectID}/subscriptions/{subscriptionID}/filters/test/{eventType} [post]
func (h *Handler) TestFilter(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
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
	_, err := subRepo.FindSubscriptionByID(r.Context(), projectID, subscriptionID)
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
