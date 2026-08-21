package notifications

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// TestBuildTeamsAdaptiveCard_Envelope pins the exact bytes posted to a Teams
// webhook. The envelope is a contract with Microsoft, not an internal shape: a
// Workflows webhook renders the payload as a card only when contentType, the
// nested content object and the schema version are all present and spelled this
// way, so this asserts the literal JSON rather than field-by-field.
func TestBuildTeamsAdaptiveCard_Envelope(t *testing.T) {
	const alertText = `endpoint url (https://e1.example.com) has been disabled, reason for failure is "Circuit breaker threshold exceeded" with a failure rate of 100.00%, endpoint status is now inactive`

	want := `{
  "type": "message",
  "attachments": [
    {
      "contentType": "application/vnd.microsoft.card.adaptive",
      "contentUrl": null,
      "content": {
        "$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
        "type": "AdaptiveCard",
        "version": "1.4",
        "body": [
          {
            "type": "TextBlock",
            "text": "endpoint url (https://e1.example.com) has been disabled, reason for failure is \"Circuit breaker threshold exceeded\" with a failure rate of 100.00%, endpoint status is now inactive",
            "wrap": true
          }
        ]
      }
    }
  ]
}`

	got, err := json.Marshal(BuildTeamsAdaptiveCard(alertText))
	require.NoError(t, err)
	require.JSONEq(t, want, string(got))
}

func TestBuildTeamsAdaptiveCard_Truncation(t *testing.T) {
	tests := []struct {
		name          string
		textLen       int
		wantTruncated bool
	}{
		{name: "under budget", textLen: maxTeamsCardTextBytes - 1, wantTruncated: false},
		{name: "at budget", textLen: maxTeamsCardTextBytes, wantTruncated: false},
		{name: "one over budget", textLen: maxTeamsCardTextBytes + 1, wantTruncated: true},
		{name: "far over budget", textLen: 50 * 1024, wantTruncated: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Repeat("a", tc.textLen)

			card := BuildTeamsAdaptiveCard(text)
			block := card.Attachments[0].Content.Body[0].Text

			require.LessOrEqual(t, len(block), maxTeamsCardTextBytes)

			if !tc.wantTruncated {
				require.Equal(t, text, block)
				return
			}

			// Truncation must be visible: a reader has to be able to tell the
			// response body in the alert was cut rather than genuinely short.
			require.True(t, strings.HasSuffix(block, teamsTruncationNotice))
			require.True(t, strings.HasPrefix(block, "aaaa"))
		})
	}
}

// TestBuildTeamsAdaptiveCard_WorstCaseSerializedSize is the reason the budget is
// a byte count rather than a rune count. Every byte here needs a six byte \uXXXX
// escape, which is the most JSON encoding can cost, so this is the largest body
// the builder can produce for any input.
func TestBuildTeamsAdaptiveCard_WorstCaseSerializedSize(t *testing.T) {
	const teamsMessageLimit = 28 * 1024

	body, err := json.Marshal(BuildTeamsAdaptiveCard(strings.Repeat("\x01", 50*1024)))
	require.NoError(t, err)
	require.Less(t, len(body), teamsMessageLimit)
}

// TestTruncateTeamsCardText_RuneBoundary asserts the cut never splits a
// multi-byte character, which would put an invalid UTF-8 byte on the wire.
func TestTruncateTeamsCardText_RuneBoundary(t *testing.T) {
	// Three-byte runes do not divide the budget evenly, so the naive cut lands
	// mid-rune and the guard has to walk back.
	text := strings.Repeat("あ", maxTeamsCardTextBytes)

	got := truncateTeamsCardText(text)

	require.True(t, strings.HasSuffix(got, teamsTruncationNotice))
	require.True(t, utf8.ValidString(got))
	require.LessOrEqual(t, len(got), maxTeamsCardTextBytes)
}
