package listener

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
)

func TestGetMetaEventDelivery_CopiesAcknowledgedAt(t *testing.T) {
	ackedAt := time.Now().UTC()
	eventDelivery := &datastore.EventDelivery{
		UID:            "ed-1",
		ProjectID:      "proj-1",
		Status:         datastore.SuccessEventStatus,
		AcknowledgedAt: null.TimeFrom(ackedAt),
	}

	meta := getMetaEventDelivery(eventDelivery)

	require.True(t, meta.AcknowledgedAt.Valid)
	require.True(t, meta.AcknowledgedAt.Time.Equal(ackedAt))
}

func TestMetaEventDelivery_PayloadIncludesTimestamps(t *testing.T) {
	requestedAt := time.Now().UTC()
	respondedAt := requestedAt.Add(40 * time.Millisecond)

	meta := getMetaEventDelivery(&datastore.EventDelivery{
		UID:            "ed-2",
		ProjectID:      "proj-1",
		Status:         datastore.SuccessEventStatus,
		AcknowledgedAt: null.TimeFrom(requestedAt),
	})
	meta.DeliveryAttempt = datastore.DeliveryAttempt{
		UID:         "att-1",
		RequestedAt: null.TimeFrom(requestedAt),
		RespondedAt: null.TimeFrom(respondedAt),
	}

	raw, err := json.Marshal(meta)
	require.NoError(t, err)

	var payload struct {
		AcknowledgedAt *time.Time `json:"acknowledged_at"`
		Attempt        struct {
			RequestedAt *time.Time `json:"requested_at"`
			RespondedAt *time.Time `json:"responded_at"`
		} `json:"attempt"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))

	require.NotNil(t, payload.AcknowledgedAt)
	require.NotNil(t, payload.Attempt.RequestedAt)
	require.NotNil(t, payload.Attempt.RespondedAt)
}
