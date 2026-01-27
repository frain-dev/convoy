package subscriptions

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"gopkg.in/guregu/null.v4"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/common"
	"github.com/frain-dev/convoy/internal/subscriptions/repo"
	"github.com/frain-dev/convoy/pkg/flatten"
	"github.com/frain-dev/convoy/util"
)

// ============================================================================
// Config Conversion Helpers
// ============================================================================

// alertConfigToParams converts AlertConfiguration to database parameters
func alertConfigToParams(ac *datastore.AlertConfiguration) (int32, string) {
	if ac == nil {
		return 0, ""
	}
	return int32(ac.Count), ac.Threshold
}

// paramsToAlertConfig converts database parameters to AlertConfiguration
func paramsToAlertConfig(count int32, threshold string) *datastore.AlertConfiguration {
	if count == 0 && threshold == "" {
		return nil
	}
	return &datastore.AlertConfiguration{
		Count:     int(count),
		Threshold: threshold,
	}
}

// retryConfigToParams converts RetryConfiguration to database parameters
func retryConfigToParams(rc *datastore.RetryConfiguration) (string, int32, int32) {
	if rc == nil {
		return "", 0, 0
	}
	return string(rc.Type), int32(rc.Duration), int32(rc.RetryCount)
}

// paramsToRetryConfig converts database parameters to RetryConfiguration
func paramsToRetryConfig(configType string, duration int32, retryCount int32) *datastore.RetryConfiguration {
	if configType == "" && duration == 0 && retryCount == 0 {
		return nil
	}
	return &datastore.RetryConfiguration{
		Type:       datastore.StrategyProvider(configType),
		Duration:   uint64(duration),
		RetryCount: uint64(retryCount),
	}
}

// filterConfigToParams converts FilterConfiguration to database parameters
func filterConfigToParams(fc *datastore.FilterConfiguration) ([]string, []byte, []byte, pgtype.Bool, []byte, []byte) {
	if fc == nil {
		return []string{}, []byte("{}"), []byte("{}"), pgtype.Bool{Bool: false, Valid: true}, []byte("{}"), []byte("{}")
	}

	eventTypes := fc.EventTypes
	if eventTypes == nil {
		eventTypes = []string{}
	}

	headers := mToPgJSON(fc.Filter.Headers)
	body := mToPgJSON(fc.Filter.Body)
	rawHeaders := mToPgJSON(fc.Filter.RawHeaders)
	rawBody := mToPgJSON(fc.Filter.RawBody)
	isFlattened := pgtype.Bool{Bool: fc.Filter.IsFlattened, Valid: true}

	return eventTypes, headers, body, isFlattened, rawHeaders, rawBody
}

// paramsToFilterConfig converts database parameters to FilterConfiguration
func paramsToFilterConfig(eventTypes []string, headers, body []byte, isFlattened pgtype.Bool, rawHeaders, rawBody []byte) *datastore.FilterConfiguration {
	if len(eventTypes) == 0 && len(headers) == 0 && len(body) == 0 {
		return &datastore.FilterConfiguration{
			EventTypes: []string{},
			Filter: datastore.FilterSchema{
				Headers:     make(datastore.M),
				Body:        make(datastore.M),
				RawHeaders:  make(datastore.M),
				RawBody:     make(datastore.M),
				IsFlattened: false,
			},
		}
	}

	return &datastore.FilterConfiguration{
		EventTypes: eventTypes,
		Filter: datastore.FilterSchema{
			Headers:     pgJSONToM(headers),
			Body:        pgJSONToM(body),
			RawHeaders:  pgJSONToM(rawHeaders),
			RawBody:     pgJSONToM(rawBody),
			IsFlattened: isFlattened.Bool,
		},
	}
}

// rateLimitConfigToParams converts RateLimitConfiguration to database parameters
func rateLimitConfigToParams(rlc *datastore.RateLimitConfiguration) (int32, int32) {
	if rlc == nil {
		return 0, 0
	}
	return int32(rlc.Count), int32(rlc.Duration)
}

// paramsToRateLimitConfig converts database parameters to RateLimitConfiguration
func paramsToRateLimitConfig(count int32, duration int32) *datastore.RateLimitConfiguration {
	if count == 0 && duration == 0 {
		return nil
	}
	return &datastore.RateLimitConfiguration{
		Count:    int(count),
		Duration: uint64(duration),
	}
}

// ============================================================================
// JSONB Conversion Helpers (using common helpers)
// ============================================================================

// mToPgJSON converts datastore.M (map) to JSONB bytes
func mToPgJSON(m datastore.M) []byte {
	return common.MToPgJSON(m)
}

// pgJSONToM converts JSONB bytes to datastore.M (map)
func pgJSONToM(data []byte) datastore.M {
	return common.PgJSONToM(data)
}

// pgJSONToFlattenM converts JSONB bytes to flatten.M (map)
func pgJSONToFlattenM(data []byte) flatten.M {
	if len(data) == 0 {
		return make(flatten.M)
	}
	var result flatten.M
	err := json.Unmarshal(data, &result)
	if err != nil {
		return make(flatten.M)
	}
	return result
}

// ============================================================================
// Type Conversion Helpers (using common helpers)
// ============================================================================

// pgTextToString converts pgtype.Text to string
func pgTextToString(pt pgtype.Text) string {
	return common.PgTextToString(pt)
}

// stringToPgText converts string to pgtype.Text
func stringToPgText(s string) pgtype.Text {
	return common.StringToPgText(s)
}

// pgTimestamptzToTime converts pgtype.Timestamptz to time.Time
func pgTimestamptzToTime(ts pgtype.Timestamptz) time.Time {
	return common.PgTimestamptzToTime(ts)
}

// deliveryModeToString converts repo.NullConvoyDeliveryMode to string
func deliveryModeToString(dm repo.NullConvoyDeliveryMode) string {
	if !dm.Valid {
		return string(datastore.AtLeastOnceDeliveryMode)
	}
	return string(dm.ConvoyDeliveryMode)
}

// stringToDeliveryMode converts string to datastore.DeliveryMode
func stringToDeliveryMode(s string) datastore.DeliveryMode {
	if s == "" {
		return datastore.AtLeastOnceDeliveryMode
	}
	return datastore.DeliveryMode(s)
}

// ============================================================================
// Metadata Conversion Helpers
// ============================================================================

// buildEndpointMetadata constructs Endpoint from row data
func buildEndpointMetadata(
	id, name, projectID, supportEmail, url, status, ownerID string,
	secrets []byte,
) *datastore.Endpoint {
	if id == "" {
		return nil
	}

	var secretsSlice []datastore.Secret
	if len(secrets) > 0 && string(secrets) != "[]" {
		if err := json.Unmarshal(secrets, &secretsSlice); err != nil {
			secretsSlice = []datastore.Secret{}
		}
	}

	return &datastore.Endpoint{
		UID:          id,
		Name:         name,
		ProjectID:    projectID,
		SupportEmail: supportEmail,
		Url:          url,
		Status:       datastore.EndpointStatus(status),
		OwnerID:      ownerID,
		Secrets:      secretsSlice,
	}
}

// buildDeviceMetadata constructs Device from row data
func buildDeviceMetadata(id, status, hostName string) *datastore.Device {
	if id == "" {
		return nil
	}

	return &datastore.Device{
		UID:      id,
		Status:   datastore.DeviceStatus(status),
		HostName: hostName,
	}
}

// buildSourceMetadata constructs Source from row data
func buildSourceMetadata(
	id, name, sourceType, maskID, projectID string,
	isDisabled bool,
	verifierType, verifierBasicUsername, verifierBasicPassword,
	verifierAPIKeyHeaderName, verifierAPIKeyHeaderValue,
	verifierHmacHash, verifierHmacHeader, verifierHmacSecret, verifierHmacEncoding string,
) *datastore.Source {
	if id == "" {
		return nil
	}

	source := &datastore.Source{
		UID:        id,
		Name:       name,
		Type:       datastore.SourceType(sourceType),
		MaskID:     maskID,
		ProjectID:  projectID,
		IsDisabled: isDisabled,
	}

	// Build verifier config if verifier type is present
	if !util.IsStringEmpty(verifierType) {
		source.Verifier = buildVerifierConfig(
			verifierType,
			verifierBasicUsername, verifierBasicPassword,
			verifierAPIKeyHeaderName, verifierAPIKeyHeaderValue,
			verifierHmacHash, verifierHmacHeader, verifierHmacSecret, verifierHmacEncoding,
		)
	}

	return source
}

// buildVerifierConfig constructs VerifierConfig from row data
func buildVerifierConfig(
	verifierType,
	basicUser, basicPass,
	apiKeyHeader, apiKeyValue,
	hmacHash, hmacHeader, hmacSecret, hmacEncoding string,
) *datastore.VerifierConfig {
	if util.IsStringEmpty(verifierType) {
		return &datastore.VerifierConfig{Type: datastore.NoopVerifier}
	}

	config := &datastore.VerifierConfig{
		Type: datastore.VerifierType(verifierType),
	}

	switch config.Type {
	case datastore.APIKeyVerifier:
		config.ApiKey = &datastore.ApiKey{
			HeaderName:  apiKeyHeader,
			HeaderValue: apiKeyValue,
		}
	case datastore.BasicAuthVerifier:
		config.BasicAuth = &datastore.BasicAuth{
			UserName: basicUser,
			Password: basicPass,
		}
	case datastore.HMacVerifier:
		config.HMac = &datastore.HMac{
			Hash:     hmacHash,
			Header:   hmacHeader,
			Secret:   hmacSecret,
			Encoding: datastore.EncodingType(hmacEncoding),
		}
	}

	return config
}

// ============================================================================
// Row to Model Conversion
// ============================================================================

// rowToSubscription converts a query row to datastore.Subscription
func rowToSubscription(row interface{}) (*datastore.Subscription, error) {
	var (
		id, name, subType, projectID                                    string
		endpointID, deviceID, sourceID                                  string
		createdAt, updatedAt                                            pgtype.Timestamptz
		function                                                        pgtype.Text
		deliveryMode                                                    repo.NullConvoyDeliveryMode
		alertConfigCount                                                int32
		alertConfigThreshold                                            string
		retryConfigType                                                 string
		retryConfigDuration, retryConfigRetryCount                      int32
		filterConfigEventTypes                                          []string
		filterConfigFilterRawHeaders, filterConfigFilterRawBody         []byte
		filterConfigFilterIsFlattened                                   pgtype.Bool
		filterConfigFilterHeaders, filterConfigFilterBody               []byte
		rateLimitConfigCount, rateLimitConfigDuration                   int32
		endpointMetadataID, endpointMetadataName                        string
		endpointMetadataProjectID, endpointMetadataSupportEmail         string
		endpointMetadataUrl, endpointMetadataStatus                     string
		endpointMetadataOwnerID                                         string
		endpointMetadataSecrets                                         []byte
		deviceMetadataID, deviceMetadataStatus                          string
		deviceMetadataHostName                                          string
		sourceMetadataID, sourceMetadataName                            string
		sourceMetadataType, sourceMetadataMaskID                        string
		sourceMetadataProjectID                                         string
		sourceMetadataIsDisabled                                        bool
		sourceVerifierType                                              string
		sourceVerifierBasicUsername, sourceVerifierBasicPassword        string
		sourceVerifierAPIKeyHeaderName, sourceVerifierAPIKeyHeaderValue string
		sourceVerifierHmacHash, sourceVerifierHmacHeader                string
		sourceVerifierHmacSecret, sourceVerifierHmacEncoding            string
	)

	// Extract fields based on row type
	switch r := row.(type) {
	case repo.FetchSubscriptionByIDRow:
		id, name, subType, projectID = r.ID, r.Name, r.Type, r.ProjectID
		endpointID, deviceID, sourceID = r.EndpointID, r.DeviceID, r.SourceID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		function = r.Function
		deliveryMode = r.DeliveryMode
		alertConfigCount, alertConfigThreshold = r.AlertConfigCount, r.AlertConfigThreshold
		retryConfigType, retryConfigDuration, retryConfigRetryCount = r.RetryConfigType, r.RetryConfigDuration, r.RetryConfigRetryCount
		filterConfigEventTypes = r.FilterConfigEventTypes
		filterConfigFilterRawHeaders, filterConfigFilterRawBody = r.FilterConfigFilterRawHeaders, r.FilterConfigFilterRawBody
		filterConfigFilterIsFlattened = r.FilterConfigFilterIsFlattened
		filterConfigFilterHeaders, filterConfigFilterBody = r.FilterConfigFilterHeaders, r.FilterConfigFilterBody
		rateLimitConfigCount, rateLimitConfigDuration = r.RateLimitConfigCount, r.RateLimitConfigDuration
		endpointMetadataID, endpointMetadataName = r.EndpointMetadataID, r.EndpointMetadataName
		endpointMetadataProjectID, endpointMetadataSupportEmail = r.EndpointMetadataProjectID, r.EndpointMetadataSupportEmail
		endpointMetadataUrl, endpointMetadataStatus = r.EndpointMetadataUrl, r.EndpointMetadataStatus
		endpointMetadataOwnerID = r.EndpointMetadataOwnerID
		endpointMetadataSecrets = r.EndpointMetadataSecrets
		deviceMetadataID, deviceMetadataStatus = r.DeviceMetadataID, r.DeviceMetadataStatus
		deviceMetadataHostName = r.DeviceMetadataHostName
		sourceMetadataID, sourceMetadataName = r.SourceMetadataID, r.SourceMetadataName
		sourceMetadataType, sourceMetadataMaskID = r.SourceMetadataType, r.SourceMetadataMaskID
		sourceMetadataProjectID = r.SourceMetadataProjectID
		sourceMetadataIsDisabled = r.SourceMetadataIsDisabled
		sourceVerifierType = r.SourceVerifierType
		sourceVerifierBasicUsername, sourceVerifierBasicPassword = r.SourceVerifierBasicUsername, r.SourceVerifierBasicPassword
		sourceVerifierAPIKeyHeaderName, sourceVerifierAPIKeyHeaderValue = r.SourceVerifierApiKeyHeaderName, r.SourceVerifierApiKeyHeaderValue
		sourceVerifierHmacHash, sourceVerifierHmacHeader = r.SourceVerifierHmacHash, r.SourceVerifierHmacHeader
		sourceVerifierHmacSecret, sourceVerifierHmacEncoding = r.SourceVerifierHmacSecret, r.SourceVerifierHmacEncoding

	case repo.FetchSubscriptionsBySourceIDRow:
		id, name, subType, projectID = r.ID, r.Name, r.Type, r.ProjectID
		endpointID, deviceID, sourceID = r.EndpointID, r.DeviceID, r.SourceID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		function = r.Function
		deliveryMode = r.DeliveryMode
		alertConfigCount, alertConfigThreshold = r.AlertConfigCount, r.AlertConfigThreshold
		retryConfigType, retryConfigDuration, retryConfigRetryCount = r.RetryConfigType, r.RetryConfigDuration, r.RetryConfigRetryCount
		filterConfigEventTypes = r.FilterConfigEventTypes
		filterConfigFilterRawHeaders, filterConfigFilterRawBody = r.FilterConfigFilterRawHeaders, r.FilterConfigFilterRawBody
		filterConfigFilterIsFlattened = r.FilterConfigFilterIsFlattened
		filterConfigFilterHeaders, filterConfigFilterBody = r.FilterConfigFilterHeaders, r.FilterConfigFilterBody
		rateLimitConfigCount, rateLimitConfigDuration = r.RateLimitConfigCount, r.RateLimitConfigDuration
		endpointMetadataID, endpointMetadataName = r.EndpointMetadataID, r.EndpointMetadataName
		endpointMetadataProjectID, endpointMetadataSupportEmail = r.EndpointMetadataProjectID, r.EndpointMetadataSupportEmail
		endpointMetadataUrl, endpointMetadataStatus = r.EndpointMetadataUrl, r.EndpointMetadataStatus
		endpointMetadataOwnerID = r.EndpointMetadataOwnerID
		endpointMetadataSecrets = r.EndpointMetadataSecrets
		deviceMetadataID, deviceMetadataStatus = r.DeviceMetadataID, r.DeviceMetadataStatus
		deviceMetadataHostName = r.DeviceMetadataHostName
		sourceMetadataID, sourceMetadataName = r.SourceMetadataID, r.SourceMetadataName
		sourceMetadataType, sourceMetadataMaskID = r.SourceMetadataType, r.SourceMetadataMaskID
		sourceMetadataProjectID = r.SourceMetadataProjectID
		sourceMetadataIsDisabled = r.SourceMetadataIsDisabled
		sourceVerifierType = r.SourceVerifierType
		sourceVerifierBasicUsername, sourceVerifierBasicPassword = r.SourceVerifierBasicUsername, r.SourceVerifierBasicPassword
		sourceVerifierAPIKeyHeaderName, sourceVerifierAPIKeyHeaderValue = r.SourceVerifierApiKeyHeaderName, r.SourceVerifierApiKeyHeaderValue
		sourceVerifierHmacHash, sourceVerifierHmacHeader = r.SourceVerifierHmacHash, r.SourceVerifierHmacHeader
		sourceVerifierHmacSecret, sourceVerifierHmacEncoding = r.SourceVerifierHmacSecret, r.SourceVerifierHmacEncoding

	case repo.FetchSubscriptionsByEndpointIDRow:
		id, name, subType, projectID = r.ID, r.Name, r.Type, r.ProjectID
		endpointID, deviceID, sourceID = r.EndpointID, r.DeviceID, r.SourceID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		function = r.Function
		deliveryMode = r.DeliveryMode
		alertConfigCount, alertConfigThreshold = r.AlertConfigCount, r.AlertConfigThreshold
		retryConfigType, retryConfigDuration, retryConfigRetryCount = r.RetryConfigType, r.RetryConfigDuration, r.RetryConfigRetryCount
		filterConfigEventTypes = r.FilterConfigEventTypes
		filterConfigFilterRawHeaders, filterConfigFilterRawBody = r.FilterConfigFilterRawHeaders, r.FilterConfigFilterRawBody
		filterConfigFilterIsFlattened = r.FilterConfigFilterIsFlattened
		filterConfigFilterHeaders, filterConfigFilterBody = r.FilterConfigFilterHeaders, r.FilterConfigFilterBody
		rateLimitConfigCount, rateLimitConfigDuration = r.RateLimitConfigCount, r.RateLimitConfigDuration
		endpointMetadataID, endpointMetadataName = r.EndpointMetadataID, r.EndpointMetadataName
		endpointMetadataProjectID, endpointMetadataSupportEmail = r.EndpointMetadataProjectID, r.EndpointMetadataSupportEmail
		endpointMetadataUrl, endpointMetadataStatus = r.EndpointMetadataUrl, r.EndpointMetadataStatus
		endpointMetadataOwnerID = r.EndpointMetadataOwnerID
		endpointMetadataSecrets = r.EndpointMetadataSecrets
		deviceMetadataID, deviceMetadataStatus = r.DeviceMetadataID, r.DeviceMetadataStatus
		deviceMetadataHostName = r.DeviceMetadataHostName
		sourceMetadataID, sourceMetadataName = r.SourceMetadataID, r.SourceMetadataName
		sourceMetadataType, sourceMetadataMaskID = r.SourceMetadataType, r.SourceMetadataMaskID
		sourceMetadataProjectID = r.SourceMetadataProjectID
		sourceMetadataIsDisabled = r.SourceMetadataIsDisabled
		sourceVerifierType = r.SourceVerifierType
		sourceVerifierBasicUsername, sourceVerifierBasicPassword = r.SourceVerifierBasicUsername, r.SourceVerifierBasicPassword
		sourceVerifierAPIKeyHeaderName, sourceVerifierAPIKeyHeaderValue = r.SourceVerifierApiKeyHeaderName, r.SourceVerifierApiKeyHeaderValue
		sourceVerifierHmacHash, sourceVerifierHmacHeader = r.SourceVerifierHmacHash, r.SourceVerifierHmacHeader
		sourceVerifierHmacSecret, sourceVerifierHmacEncoding = r.SourceVerifierHmacSecret, r.SourceVerifierHmacEncoding

	case repo.FetchSubscriptionByDeviceIDRow:
		id, name, subType, projectID = r.ID, r.Name, r.Type, r.ProjectID
		endpointID, deviceID, sourceID = r.EndpointID, r.DeviceID, r.SourceID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		function = r.Function
		deliveryMode = r.DeliveryMode
		alertConfigCount, alertConfigThreshold = r.AlertConfigCount, r.AlertConfigThreshold
		retryConfigType, retryConfigDuration, retryConfigRetryCount = r.RetryConfigType, r.RetryConfigDuration, r.RetryConfigRetryCount
		filterConfigEventTypes = r.FilterConfigEventTypes
		filterConfigFilterRawHeaders, filterConfigFilterRawBody = r.FilterConfigFilterRawHeaders, r.FilterConfigFilterRawBody
		filterConfigFilterIsFlattened = r.FilterConfigFilterIsFlattened
		filterConfigFilterHeaders, filterConfigFilterBody = r.FilterConfigFilterHeaders, r.FilterConfigFilterBody
		rateLimitConfigCount, rateLimitConfigDuration = r.RateLimitConfigCount, r.RateLimitConfigDuration
		deviceMetadataID, deviceMetadataStatus = r.DeviceMetadataID, r.DeviceMetadataStatus
		deviceMetadataHostName = r.DeviceMetadataHostName

	case repo.FetchCLISubscriptionsRow:
		id, name, subType, projectID = r.ID, r.Name, r.Type, r.ProjectID
		endpointID, deviceID, sourceID = r.EndpointID, r.DeviceID, r.SourceID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		function = r.Function
		deliveryMode = r.DeliveryMode
		alertConfigCount, alertConfigThreshold = r.AlertConfigCount, r.AlertConfigThreshold
		retryConfigType, retryConfigDuration, retryConfigRetryCount = r.RetryConfigType, r.RetryConfigDuration, r.RetryConfigRetryCount
		filterConfigEventTypes = r.FilterConfigEventTypes
		filterConfigFilterRawHeaders, filterConfigFilterRawBody = r.FilterConfigFilterRawHeaders, r.FilterConfigFilterRawBody
		filterConfigFilterIsFlattened = r.FilterConfigFilterIsFlattened
		filterConfigFilterHeaders, filterConfigFilterBody = r.FilterConfigFilterHeaders, r.FilterConfigFilterBody
		rateLimitConfigCount, rateLimitConfigDuration = r.RateLimitConfigCount, r.RateLimitConfigDuration
		endpointMetadataID, endpointMetadataName = r.EndpointMetadataID, r.EndpointMetadataName
		endpointMetadataProjectID, endpointMetadataSupportEmail = r.EndpointMetadataProjectID, r.EndpointMetadataSupportEmail
		endpointMetadataUrl, endpointMetadataStatus = r.EndpointMetadataUrl, r.EndpointMetadataStatus
		endpointMetadataOwnerID = r.EndpointMetadataOwnerID
		endpointMetadataSecrets = r.EndpointMetadataSecrets
		deviceMetadataID, deviceMetadataStatus = r.DeviceMetadataID, r.DeviceMetadataStatus
		deviceMetadataHostName = r.DeviceMetadataHostName
		sourceMetadataID, sourceMetadataName = r.SourceMetadataID, r.SourceMetadataName
		sourceMetadataType, sourceMetadataMaskID = r.SourceMetadataType, r.SourceMetadataMaskID
		sourceMetadataProjectID = r.SourceMetadataProjectID
		sourceMetadataIsDisabled = r.SourceMetadataIsDisabled
		sourceVerifierType = r.SourceVerifierType
		sourceVerifierBasicUsername, sourceVerifierBasicPassword = r.SourceVerifierBasicUsername, r.SourceVerifierBasicPassword
		sourceVerifierAPIKeyHeaderName, sourceVerifierAPIKeyHeaderValue = r.SourceVerifierApiKeyHeaderName, r.SourceVerifierApiKeyHeaderValue
		sourceVerifierHmacHash, sourceVerifierHmacHeader = r.SourceVerifierHmacHash, r.SourceVerifierHmacHeader
		sourceVerifierHmacSecret, sourceVerifierHmacEncoding = r.SourceVerifierHmacSecret, r.SourceVerifierHmacEncoding

	case repo.FetchSubscriptionsPaginatedRow:
		id, name, subType, projectID = r.ID, r.Name, r.Type, r.ProjectID
		endpointID, deviceID, sourceID = r.EndpointID, r.DeviceID, r.SourceID
		createdAt, updatedAt = r.CreatedAt, r.UpdatedAt
		function = r.Function
		deliveryMode = r.DeliveryMode
		alertConfigCount, alertConfigThreshold = r.AlertConfigCount, r.AlertConfigThreshold
		retryConfigType, retryConfigDuration, retryConfigRetryCount = r.RetryConfigType, r.RetryConfigDuration, r.RetryConfigRetryCount
		filterConfigEventTypes = r.FilterConfigEventTypes
		filterConfigFilterRawHeaders, filterConfigFilterRawBody = r.FilterConfigFilterRawHeaders, r.FilterConfigFilterRawBody
		filterConfigFilterIsFlattened = r.FilterConfigFilterIsFlattened
		filterConfigFilterHeaders, filterConfigFilterBody = r.FilterConfigFilterHeaders, r.FilterConfigFilterBody
		rateLimitConfigCount, rateLimitConfigDuration = r.RateLimitConfigCount, r.RateLimitConfigDuration
		endpointMetadataID, endpointMetadataName = r.EndpointMetadataID, r.EndpointMetadataName
		endpointMetadataProjectID, endpointMetadataSupportEmail = r.EndpointMetadataProjectID, r.EndpointMetadataSupportEmail
		endpointMetadataUrl, endpointMetadataStatus = r.EndpointMetadataUrl, r.EndpointMetadataStatus
		endpointMetadataOwnerID = r.EndpointMetadataOwnerID
		endpointMetadataSecrets = r.EndpointMetadataSecrets
		deviceMetadataID, deviceMetadataStatus = r.DeviceMetadataID, r.DeviceMetadataStatus
		deviceMetadataHostName = r.DeviceMetadataHostName
		sourceMetadataID, sourceMetadataName = r.SourceMetadataID, r.SourceMetadataName
		sourceMetadataType, sourceMetadataMaskID = r.SourceMetadataType, r.SourceMetadataMaskID
		sourceMetadataProjectID = r.SourceMetadataProjectID
		sourceMetadataIsDisabled = r.SourceMetadataIsDisabled
		sourceVerifierType = r.SourceVerifierType
		sourceVerifierBasicUsername, sourceVerifierBasicPassword = r.SourceVerifierBasicUsername, r.SourceVerifierBasicPassword
		sourceVerifierAPIKeyHeaderName, sourceVerifierAPIKeyHeaderValue = r.SourceVerifierApiKeyHeaderName, r.SourceVerifierApiKeyHeaderValue
		sourceVerifierHmacHash, sourceVerifierHmacHeader = r.SourceVerifierHmacHash, r.SourceVerifierHmacHeader
		sourceVerifierHmacSecret, sourceVerifierHmacEncoding = r.SourceVerifierHmacSecret, r.SourceVerifierHmacEncoding

	default:
		return nil, nil
	}

	// Build subscription model
	subscription := &datastore.Subscription{
		UID:          id,
		Name:         name,
		Type:         datastore.SubscriptionType(subType),
		ProjectID:    projectID,
		EndpointID:   endpointID,
		DeviceID:     deviceID,
		SourceID:     sourceID,
		Function:     null.NewString(pgTextToString(function), function.Valid),
		DeliveryMode: stringToDeliveryMode(deliveryModeToString(deliveryMode)),
		CreatedAt:    pgTimestamptzToTime(createdAt),
		UpdatedAt:    pgTimestamptzToTime(updatedAt),
	}

	// Convert configs
	subscription.AlertConfig = paramsToAlertConfig(alertConfigCount, alertConfigThreshold)
	subscription.RetryConfig = paramsToRetryConfig(retryConfigType, retryConfigDuration, retryConfigRetryCount)
	subscription.FilterConfig = paramsToFilterConfig(
		filterConfigEventTypes,
		filterConfigFilterHeaders,
		filterConfigFilterBody,
		filterConfigFilterIsFlattened,
		filterConfigFilterRawHeaders,
		filterConfigFilterRawBody,
	)
	subscription.RateLimitConfig = paramsToRateLimitConfig(rateLimitConfigCount, rateLimitConfigDuration)

	// Build metadata
	subscription.Endpoint = buildEndpointMetadata(
		endpointMetadataID, endpointMetadataName, endpointMetadataProjectID,
		endpointMetadataSupportEmail, endpointMetadataUrl, endpointMetadataStatus,
		endpointMetadataOwnerID, endpointMetadataSecrets,
	)

	subscription.Device = buildDeviceMetadata(
		deviceMetadataID, deviceMetadataStatus, deviceMetadataHostName,
	)

	subscription.Source = buildSourceMetadata(
		sourceMetadataID, sourceMetadataName, sourceMetadataType,
		sourceMetadataMaskID, sourceMetadataProjectID, sourceMetadataIsDisabled,
		sourceVerifierType,
		sourceVerifierBasicUsername, sourceVerifierBasicPassword,
		sourceVerifierAPIKeyHeaderName, sourceVerifierAPIKeyHeaderValue,
		sourceVerifierHmacHash, sourceVerifierHmacHeader,
		sourceVerifierHmacSecret, sourceVerifierHmacEncoding,
	)

	return subscription, nil
}
