package task

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRetryProjectIDsUsesCallerProject(t *testing.T) {
	ids, err := retryProjectIDs(context.Background(), nil, nil, "proj_abc")
	require.NoError(t, err)
	require.Equal(t, []string{"proj_abc"}, ids)
}
