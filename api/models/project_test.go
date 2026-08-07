package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// sync_dynamic_event_ack shipped in v26.7.0 and was renamed. Decoding it as an
// unknown key would return 200 with the setting untouched, so a caller still on
// the old name would believe it toggled something.
func TestProjectConfigRejectsRenamedSyncDynamicEventAck(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "on update", body: `{"config":{"sync_dynamic_event_ack":true}}`},
		{name: "on update set to false", body: `{"config":{"sync_dynamic_event_ack":false}}`},
		{name: "alongside the new name", body: `{"config":{"verify_dynamic_events":true,"sync_dynamic_event_ack":true}}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var update UpdateProject
			err := json.Unmarshal([]byte(tt.body), &update)
			require.ErrorIs(t, err, ErrRenamedSyncDynamicEventAck)
		})
	}
}

// Create decodes the same ProjectConfig, so the guard must reject there too
// rather than only on the path it was written for.
func TestCreateProjectRejectsRenamedSyncDynamicEventAck(t *testing.T) {
	var create CreateProject
	err := json.Unmarshal([]byte(`{"name":"p","config":{"sync_dynamic_event_ack":true}}`), &create)
	require.ErrorIs(t, err, ErrRenamedSyncDynamicEventAck)
}

func TestProjectConfigDecodesDynamicEventSettings(t *testing.T) {
	var update UpdateProject
	err := json.Unmarshal([]byte(`{"config":{"verify_dynamic_events":true,"allow_unmatched_dynamic_urls":true}}`), &update)
	require.NoError(t, err)
	require.NotNil(t, update.Config)
	require.True(t, update.Config.VerifyDynamicEvents)
	require.True(t, update.Config.AllowUnmatchedDynamicURLs)

	// The present-key map drives the patch merge, so both must survive the
	// custom ProjectConfig unmarshaller.
	present := update.ConfigPresentKeys()
	require.Contains(t, present, "verify_dynamic_events")
	require.Contains(t, present, "allow_unmatched_dynamic_urls")
}

// A config body that omits the settings must leave them absent from the present
// keys, so the merge does not overwrite a project's stored values with false.
func TestProjectConfigOmittedSettingsAreNotPresent(t *testing.T) {
	var update UpdateProject
	err := json.Unmarshal([]byte(`{"config":{"disable_endpoint":true}}`), &update)
	require.NoError(t, err)

	present := update.ConfigPresentKeys()
	require.NotContains(t, present, "verify_dynamic_events")
	require.NotContains(t, present, "allow_unmatched_dynamic_urls")
}
