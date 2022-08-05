package httpheader

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_MergeHeaders(t *testing.T) {
	tests := map[string]struct {
		header    HTTPHeader
		newHeader HTTPHeader
		fields    []string
	}{
		"merge_new_fields": {
			header: HTTPHeader(map[string][]string{
				"X-Convoy-Signature": []string{"MDQ6VXNlcjgwNzQwMTE1"},
			}),
			newHeader: HTTPHeader(map[string][]string{
				"X-GitHub-Delivery": []string{"c2520a2e-121b-11ed-862c-d3f38c5356fa"},
				"X-GitHub-Event":    []string{"issue_comment"},
				"X-GitHub-Hook-ID":  []string{"355729303"},
			}),
			fields: []string{"X-GitHub-Delivery", "X-GitHub-Event", "X-GitHub-Hook-ID"},
		},
		"do_not_overwrite_old_fields": {
			header: HTTPHeader(map[string][]string{
				"User-Agent": []string{"Convoy v0.6"},
			}),
			newHeader: HTTPHeader(map[string][]string{
				"User-Agent": []string{"GitHub-Hookshot/9398d35"},
			}),
			fields: []string{"User-Agent"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Arrange.
			headerClone := http.Header(tc.header).Clone()

			// Act.
			tc.header.MergeHeaders(tc.newHeader)

			// Act.
			for i, v := range tc.fields {
				require.Contains(t, tc.header, tc.fields[i])

				_, wasPresentInHeader := headerClone[v]
				if wasPresentInHeader {
					require.NotEqual(t, headerClone[v], tc.newHeader[v])
				}
			}
		})
	}
}
