package models

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type validatable interface {
	Validate() error
}

// ingestEventCases covers every event struct on the ingest path. Each returns a
// fully populated, valid instance so a single field can be cleared per case.
func ingestEventCases() []struct {
	name  string
	valid func() validatable
} {
	return []struct {
		name  string
		valid func() validatable
	}{
		{
			name: "CreateEvent",
			valid: func() validatable {
				return &CreateEvent{
					EndpointID: "endpoint-1",
					EventType:  "invoice.paid",
					Data:       json.RawMessage(`{"name":"daniel"}`),
				}
			},
		},
		{
			name: "DynamicEvent",
			valid: func() validatable {
				return &DynamicEvent{
					URL:       "https://testing.com",
					Secret:    "12345",
					EventType: "invoice.paid",
					Data:      json.RawMessage(`{"name":"daniel"}`),
				}
			},
		},
		{
			name: "DynamicEventStub",
			valid: func() validatable {
				return &DynamicEventStub{
					EventType: "invoice.paid",
					Data:      json.RawMessage(`{"name":"daniel"}`),
				}
			},
		},
		{
			name: "BroadcastEvent",
			valid: func() validatable {
				return &BroadcastEvent{
					EventType: "invoice.paid",
					Data:      json.RawMessage(`{"name":"daniel"}`),
				}
			},
		},
		{
			name: "FanoutEvent",
			valid: func() validatable {
				return &FanoutEvent{
					OwnerID:   "owner-1",
					EventType: "invoice.paid",
					Data:      json.RawMessage(`{"name":"daniel"}`),
				}
			},
		},
	}
}

// The ingest event structs validate by hand rather than through govalidator,
// because govalidator walks every element of a slice field and json.RawMessage
// is a byte slice, so its cost scaled with the size of the webhook payload.
//
// The valid: tags stay on those structs as the declaration of what is required,
// and this test is what keeps the hand-written code in step with them: it derives
// a case from every tag and asserts the exact error string govalidator produced,
// so adding or changing a tag fails here until the hand-written path enforces it.
func TestIngestEventValidateEnforcesEveryValidTag(t *testing.T) {
	for _, tc := range ingestEventCases() {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.valid().Validate(), "the case itself must be valid")

			structType := reflect.TypeOf(tc.valid()).Elem()
			checked := 0

			for i := range structType.NumField() {
				field := structType.Field(i)

				tag := field.Tag.Get("valid")
				if tag == "" {
					continue
				}

				rule, message, found := strings.Cut(tag, "~")
				require.True(t, found,
					"%s.%s: valid tag %q carries no ~message, so this test cannot derive the expected error. Teach it the new tag shape.",
					tc.name, field.Name, tag)

				jsonName, _, _ := strings.Cut(field.Tag.Get("json"), ",")
				require.NotEmpty(t, jsonName, "%s.%s: needs a json name to report against", tc.name, field.Name)

				cleared := tc.valid()
				reflect.ValueOf(cleared).Elem().Field(i).SetZero()
				err := cleared.Validate()

				switch rule {
				case "required":
					require.EqualError(t, err, jsonName+":"+message,
						"%s.%s is tagged required but clearing it did not produce that rejection", tc.name, field.Name)
				case "optional":
					require.NoError(t, err,
						"%s.%s is tagged optional but clearing it was rejected", tc.name, field.Name)
				default:
					t.Fatalf("%s.%s: valid tag rule %q is not one this test knows how to check. Teach it the new rule.",
						tc.name, field.Name, rule)
				}

				checked++
			}

			require.Positive(t, checked, "%s: no valid tags were exercised, so this test proves nothing", tc.name)
		})
	}
}

// An endpoint event must still name its delivery target, which is checked after
// the required fields rather than by a tag.
func TestCreateEventValidateOrdersDataBeforeDeliveryTarget(t *testing.T) {
	noTarget := &CreateEvent{EventType: "invoice.paid", Data: json.RawMessage(`{"a":1}`)}
	require.EqualError(t, noTarget.Validate(), "please provide an endpoint ID")

	// Data is reported first, as it was when govalidator ran ahead of this check.
	noTargetNoData := &CreateEvent{EventType: "invoice.paid"}
	require.EqualError(t, noTargetNoData.Validate(), "data:please provide your data")
}

// Every missing required field is reported, not just the first one, and in
// field declaration order. util.Validate built this string by ranging over a
// map, so the order it reported was random.
func TestIngestEventValidateReportsEveryMissingField(t *testing.T) {
	require.EqualError(t, (&FanoutEvent{}).Validate(),
		"owner_id:please provide an owner id, event_type:please provide an event type, data:please provide your data")
}

// Validation must not walk the payload. json.RawMessage is a byte slice, and
// govalidator's slice branch ran the whole validator machinery per element, so
// cost grew at roughly 560ns per payload byte, measured here as 580ms to 615ms
// for the 1MB below. The hand-written path is payload-size independent and needs
// microseconds, so this budget has several orders of magnitude of headroom and
// only trips if the per-byte walk comes back.
func TestIngestEventValidateDoesNotWalkPayload(t *testing.T) {
	const budget = 50 * time.Millisecond

	payload := json.RawMessage(`{"name":"` + strings.Repeat("a", 1<<20) + `"}`)

	for _, tc := range ingestEventCases() {
		t.Run(tc.name, func(t *testing.T) {
			event := tc.valid()
			reflect.ValueOf(event).Elem().FieldByName("Data").Set(reflect.ValueOf(payload))

			start := time.Now()
			require.NoError(t, event.Validate())
			elapsed := time.Since(start)

			require.Less(t, elapsed, budget,
				"validating a 1MB payload took %s, which means cost is scaling with payload size again", elapsed)
		})
	}
}
