package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/events"
	"github.com/frain-dev/convoy/internal/pkg/dedup"
	"github.com/frain-dev/convoy/internal/pkg/middleware"
	"github.com/frain-dev/convoy/internal/sources"
	"github.com/frain-dev/convoy/util"
)

// maxIdempotencyValueEvents bounds how many events one idempotencyValue can match.
const maxIdempotencyValueEvents = 1000

const idempotencySourcePageSize = 100

var (
	errNoIdempotencyValueSource   = errors.New("idempotencyValue needs an http source with exactly one idempotency key location")
	errIdempotencyValuePortalLink = errors.New("idempotencyValue is not supported for portal links")
)

// rejectPortalIdempotencyValue refuses idempotencyValue for portal links. Resolution
// reads project-wide sources and events, so its answers (no match, unknown source)
// would reveal data outside the link's endpoints.
func (h *Handler) rejectPortalIdempotencyValue(r *http.Request, value string) error {
	if value != "" && h.IsReqWithPortalLinkToken(middleware.GetAuthUserFromContext(r.Context())) {
		return util.NewServiceError(http.StatusBadRequest, errIdempotencyValuePortalLink)
	}
	return nil
}

// applyIdempotencyValue narrows data to the events matched by its idempotencyValue.
// It returns false when the value matched no event; callers must then answer with an
// empty result, because an empty EventIDs filter means "every event".
func (h *Handler) applyIdempotencyValue(r *http.Request, projectID string, data *models.QueryListEventDeliveryResponse) (bool, error) {
	if data.IdempotencyValue == "" {
		return true, nil
	}
	if err := h.rejectPortalIdempotencyValue(r, data.IdempotencyValue); err != nil {
		return false, err
	}

	ids, err := h.idempotencyValueEventIDs(r.Context(), projectID, data.IdempotencyValue, data.SourceIDs)
	if err != nil {
		return false, err
	}
	if len(ids) == 0 {
		return false, nil
	}

	data.Filter.SearchParams.EventIDs = ids
	return true, nil
}

// idempotencyValueKeys returns the stored idempotency key value would produce in each
// of the given sources, or in every http source in the project when sourceIDs is empty.
// Only sources with a single idempotency key location qualify, since the stored key
// for multi-location sources cannot be derived from one value.
func (h *Handler) idempotencyValueKeys(ctx context.Context, projectID, value string, sourceIDs []string) ([]string, error) {
	candidates, err := h.idempotencyValueSources(ctx, projectID, sourceIDs)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(candidates))
	for i := range candidates {
		if candidates[i].Type == datastore.HTTPSource && len(candidates[i].IdempotencyKeys) == 1 {
			keys = append(keys, dedup.ChecksumForValue(candidates[i].Name, value))
		}
	}
	if len(keys) == 0 {
		return nil, util.NewServiceError(http.StatusBadRequest, errNoIdempotencyValueSource)
	}
	return keys, nil
}

// idempotencyValueEventIDs returns up to maxIdempotencyValueEvents events ingested with value.
func (h *Handler) idempotencyValueEventIDs(ctx context.Context, projectID, value string, sourceIDs []string) ([]string, error) {
	keys, err := h.idempotencyValueKeys(ctx, projectID, value, sourceIDs)
	if err != nil {
		return nil, err
	}

	ids, err := events.New(h.A.Logger, h.A.DB).FindEventIDsByIdempotencyKeys(ctx, projectID, keys, maxIdempotencyValueEvents)
	if err != nil {
		h.A.Logger.ErrorContext(ctx, "failed to find events by idempotency value", "error", err)
		return nil, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while resolving idempotencyValue"))
	}
	return ids, nil
}

func (h *Handler) idempotencyValueSources(ctx context.Context, projectID string, sourceIDs []string) ([]datastore.Source, error) {
	repo := sources.New(h.A.Logger, h.A.DB)

	if len(sourceIDs) > 0 {
		out := make([]datastore.Source, 0, len(sourceIDs))
		for _, id := range sourceIDs {
			source, err := repo.FindSourceByID(ctx, projectID, id)
			if err != nil {
				if errors.Is(err, datastore.ErrSourceNotFound) {
					return nil, util.NewServiceError(http.StatusBadRequest, err)
				}
				h.A.Logger.ErrorContext(ctx, "failed to find source for idempotency value", "error", err)
				return nil, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while retrieving sources"))
			}
			out = append(out, *source)
		}
		return out, nil
	}

	var out []datastore.Source
	pageable := datastore.Pageable{PerPage: idempotencySourcePageSize, Direction: datastore.Next, Sort: "DESC"}
	pageable.SetCursors()
	for {
		page, pagination, err := repo.LoadSourcesPaged(ctx, projectID, &datastore.SourceFilter{Type: string(datastore.HTTPSource)}, pageable)
		if err != nil {
			h.A.Logger.ErrorContext(ctx, "failed to load sources for idempotency value", "error", err)
			return nil, util.NewServiceError(http.StatusInternalServerError, errors.New("an error occurred while retrieving sources"))
		}
		out = append(out, page...)
		if !pagination.HasNextPage || pagination.NextPageCursor == "" {
			return out, nil
		}
		pageable.NextCursor = pagination.NextPageCursor
	}
}
