package handlers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterAllowedEndpointIDs(t *testing.T) {
	allowed := []string{"ep-a", "ep-b", "ep-c"}

	t.Run("no request returns full allowlist", func(t *testing.T) {
		require.Equal(t, allowed, filterAllowedEndpointIDs(nil, allowed))
		require.Equal(t, allowed, filterAllowedEndpointIDs([]string{}, allowed))
	})

	t.Run("keeps only requested ids that are allowed", func(t *testing.T) {
		require.Equal(t, []string{"ep-b"}, filterAllowedEndpointIDs([]string{"ep-b"}, allowed))
		require.Equal(t, []string{"ep-a", "ep-c"}, filterAllowedEndpointIDs([]string{"ep-a", "ep-c"}, allowed))
	})

	t.Run("drops ids outside the allowlist", func(t *testing.T) {
		require.Empty(t, filterAllowedEndpointIDs([]string{"ep-outside"}, allowed))
		require.Equal(t, []string{"ep-a"}, filterAllowedEndpointIDs([]string{"ep-outside", "ep-a"}, allowed))
	})

	t.Run("empty allowlist never widens", func(t *testing.T) {
		require.Empty(t, filterAllowedEndpointIDs([]string{"ep-a"}, nil))
		require.Empty(t, filterAllowedEndpointIDs([]string{"ep-a"}, []string{}))
	})
}
