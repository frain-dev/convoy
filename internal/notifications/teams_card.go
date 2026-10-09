package notifications

import "unicode/utf8"

const (
	// teamsCardContentType is the attachment content type a Workflows (Power
	// Automate) webhook needs to render the payload as a card rather than text.
	teamsCardContentType = "application/vnd.microsoft.card.adaptive"

	// teamsCardSchema and teamsCardVersion pin the Adaptive Card contract. 1.4 is
	// the highest version Teams renders across desktop, web and mobile, so the
	// card cannot degrade to a raw JSON dump on one client.
	teamsCardSchema  = "http://adaptivecards.io/schemas/adaptive-card.json"
	teamsCardVersion = "1.4"

	// maxTeamsCardTextBytes bounds the TextBlock so the serialized envelope stays
	// under the ~28KB Workflows message limit. The alert text can carry a whole
	// endpoint response body (CONVOY_MAX_RESPONSE_SIZE defaults to 50KB), and JSON
	// escaping costs up to 6 bytes per input byte, because invalid UTF-8 and
	// control characters each become a \uXXXX sequence. The budget is therefore
	// sized so even an all-escaped text stays inside the limit in one pass:
	// 4096 * 6 = 24576 bytes of string, plus a few hundred bytes of envelope.
	maxTeamsCardTextBytes = 4096

	// teamsTruncationNotice replaces the dropped tail so a reader can tell the
	// card is incomplete instead of silently reading a cut-off response body.
	teamsTruncationNotice = "\n\n[truncated: alert exceeded the Microsoft Teams message size limit]"
)

// TeamsAdaptiveCardMessage is the envelope a Teams incoming webhook accepts.
// Retired Office 365 connectors took a bare MessageCard; the Workflows webhook
// that replaced them expects this attachment wrapper.
type TeamsAdaptiveCardMessage struct {
	Type        string                `json:"type"`
	Attachments []TeamsCardAttachment `json:"attachments"`
}

type TeamsCardAttachment struct {
	ContentType string            `json:"contentType"`
	ContentURL  *string           `json:"contentUrl"`
	Content     TeamsAdaptiveCard `json:"content"`
}

type TeamsAdaptiveCard struct {
	Schema  string                  `json:"$schema"`
	Type    string                  `json:"type"`
	Version string                  `json:"version"`
	Body    []TeamsAdaptiveCardText `json:"body"`
}

type TeamsAdaptiveCardText struct {
	Type string `json:"type"`
	Text string `json:"text"`
	Wrap bool   `json:"wrap"`
}

// BuildTeamsAdaptiveCard wraps one alert string in the Adaptive Card envelope,
// truncating the text to the documented budget first.
func BuildTeamsAdaptiveCard(text string) TeamsAdaptiveCardMessage {
	return TeamsAdaptiveCardMessage{
		Type: "message",
		Attachments: []TeamsCardAttachment{
			{
				ContentType: teamsCardContentType,
				Content: TeamsAdaptiveCard{
					Schema:  teamsCardSchema,
					Type:    "AdaptiveCard",
					Version: teamsCardVersion,
					Body: []TeamsAdaptiveCardText{
						{
							Type: "TextBlock",
							Text: truncateTeamsCardText(text),
							Wrap: true,
						},
					},
				},
			},
		},
	}
}

// truncateTeamsCardText trims text to maxTeamsCardTextBytes including the
// notice, cutting on a rune boundary so the card never carries a split
// multi-byte character.
func truncateTeamsCardText(text string) string {
	if len(text) <= maxTeamsCardTextBytes {
		return text
	}

	keep := maxTeamsCardTextBytes - len(teamsTruncationNotice)
	for keep > 0 && !utf8.RuneStart(text[keep]) {
		keep--
	}

	return text[:keep] + teamsTruncationNotice
}
