package handlers

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/internal/pkg/billing"
)

func TestBillingClientErrorStatus(t *testing.T) {
	t.Parallel()

	require.Equal(t, http.StatusServiceUnavailable, billingClientErrorStatus(errors.New("upstream down"), http.StatusServiceUnavailable))
	require.Equal(t, http.StatusConflict, billingClientErrorStatus(&billing.Error{StatusCode: http.StatusConflict, Message: "conflict"}, http.StatusServiceUnavailable))
	require.Equal(t, http.StatusInternalServerError, billingClientErrorStatus(&billing.Error{StatusCode: http.StatusBadRequest, Message: "bad"}, http.StatusInternalServerError))
	require.True(t, billingClientErrorIsDefinitive(&billing.Error{StatusCode: http.StatusConflict, Message: "conflict"}))
	require.False(t, billingClientErrorIsDefinitive(&billing.Error{StatusCode: http.StatusBadGateway, Message: "upstream"}))
	require.False(t, billingClientErrorIsDefinitive(errors.New("dial tcp: timeout")))
}

func TestValidatePlanAndHost(t *testing.T) {
	t.Parallel()

	host, err := validatePlanAndHost("plan-1", "https://app.example.com")
	require.NoError(t, err)
	require.Equal(t, "https://app.example.com", host)

	_, err = validatePlanAndHost("", "https://app.example.com")
	require.Error(t, err)
	require.Contains(t, err.Error(), "plan_id is required")

	_, err = validatePlanAndHost("plan-1", "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "host is required")

	_, err = validatePlanAndHost("plan-1", "not-a-url")
	require.Error(t, err)
}

func TestValidateResubscribeEmail(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateResubscribeEmail("buyer@example.com", ""))
	require.NoError(t, validateResubscribeEmail("", "license-key"))
	require.Error(t, validateResubscribeEmail("", ""))
}

func TestOptionalCanonicalHost(t *testing.T) {
	t.Parallel()

	host, err := optionalCanonicalHost("")
	require.NoError(t, err)
	require.Equal(t, "", host)

	host, err = optionalCanonicalHost("https://app.example.com")
	require.NoError(t, err)
	require.Equal(t, "https://app.example.com", host)

	_, err = optionalCanonicalHost("bad host")
	require.Error(t, err)
}
