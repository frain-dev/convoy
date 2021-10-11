package task

import "testing"

func TestProcessMessages(t *testing.T) {
	tt := []struct {
		name string
	}{
		{
			name: "Message already sent.",
		},
		{
			name: "Endpoint is inactive",
		},
		{
			name: "Endpoint does not respond with 2xx",
		},
		{
			name: "Max retries reached and success",
		},
		{
			name: "Max retries reached and failure",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
		})
	}
}
