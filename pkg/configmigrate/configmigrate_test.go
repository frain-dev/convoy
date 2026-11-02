package configmigrate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunnerAppliesStepsInOrder(t *testing.T) {
	t.Parallel()

	var order []string
	r := New(
		func(Env, map[string]any) ([]Deprecation, error) {
			order = append(order, "a")
			return []Deprecation{{Old: "OLD_A", New: "NEW_A"}}, nil
		},
		func(Env, map[string]any) ([]Deprecation, error) {
			order = append(order, "b")
			return nil, nil
		},
	)

	deps, err := r.Apply(MapEnv{}, map[string]any{})
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b"}, order)
	require.Equal(t, []Deprecation{{Old: "OLD_A", New: "NEW_A"}}, deps)
}

func TestRenameEnvStringPrefersNew(t *testing.T) {
	t.Parallel()

	step := RenameEnvString("OLD_PERIOD", "NEW_PERIOD", func(v string) {
		t.Fatalf("setter should not run when new is set, got %q", v)
	})
	deps, err := step(MapEnv{
		"OLD_PERIOD": "720h",
		"NEW_PERIOD": "168h",
	}, nil)
	require.NoError(t, err)
	require.Empty(t, deps)
}

func TestRenameEnvStringMigratesOld(t *testing.T) {
	t.Parallel()

	var got string
	step := RenameEnvString("OLD_PERIOD", "NEW_PERIOD", func(v string) { got = v })
	deps, err := step(MapEnv{"OLD_PERIOD": "720h"}, nil)
	require.NoError(t, err)
	require.Equal(t, "720h", got)
	require.Equal(t, []Deprecation{{Old: "OLD_PERIOD", New: "NEW_PERIOD"}}, deps)
}

func TestRenameEnvBoolMigratesOld(t *testing.T) {
	t.Parallel()

	var got bool
	step := RenameEnvBool("OLD_ENABLED", "NEW_ENABLED", func(v bool) { got = v })
	deps, err := step(MapEnv{"OLD_ENABLED": "true"}, nil)
	require.NoError(t, err)
	require.True(t, got)
	require.Equal(t, []Deprecation{{Old: "OLD_ENABLED", New: "NEW_ENABLED"}}, deps)
}

func TestRenameJSONStringMigratesNested(t *testing.T) {
	t.Parallel()

	root := map[string]any{
		"retention_policy": map[string]any{
			"policy": "720h",
		},
	}
	step := RenameJSONString(
		[]string{"retention_policy", "policy"},
		[]string{"retention", "period"},
	)
	deps, err := step(MapEnv{}, root)
	require.NoError(t, err)
	require.Equal(t, "720h", Object(root, "retention")["period"])
	require.Equal(t, []Deprecation{{Old: "retention_policy.policy", New: "retention.period"}}, deps)
}

func TestRenameJSONStringSkipsWhenNewPresent(t *testing.T) {
	t.Parallel()

	root := map[string]any{
		"retention_policy": map[string]any{"policy": "720h"},
		"retention":        map[string]any{"period": "168h"},
	}
	step := RenameJSONString(
		[]string{"retention_policy", "policy"},
		[]string{"retention", "period"},
	)
	deps, err := step(MapEnv{}, root)
	require.NoError(t, err)
	require.Empty(t, deps)
	require.Equal(t, "168h", Object(root, "retention")["period"])
}
