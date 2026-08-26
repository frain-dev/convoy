// Package configmigrate applies ordered, named migrations to configuration
// sources (env and JSON) at load time. It is the boot-time counterpart to
// requestmigrations: transform old operator input into the canonical shape,
// collect deprecation notices, then drop the old keys in a later release.
package configmigrate

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Deprecation records that a legacy key was accepted and should be renamed.
type Deprecation struct {
	Old string
	New string
}

func (d Deprecation) String() string {
	return fmt.Sprintf("%s is deprecated; use %s", d.Old, d.New)
}

// Env is a read view of process environment for migrations and tests.
type Env interface {
	Lookup(key string) (string, bool)
}

// OSEnv reads from the process environment.
type OSEnv struct{}

func (OSEnv) Lookup(key string) (string, bool) { return os.LookupEnv(key) }

// MapEnv is an in-memory Env for tests.
type MapEnv map[string]string

func (m MapEnv) Lookup(key string) (string, bool) {
	v, ok := m[key]
	return v, ok
}

// Step renames or reshapes one concern. Steps run in registration order.
// They must be idempotent when the new keys are already set.
type Step func(env Env, jsonRoot map[string]any) ([]Deprecation, error)

// Runner applies a list of Steps once at config load.
type Runner struct {
	steps []Step
}

func New(steps ...Step) *Runner {
	return &Runner{steps: append([]Step(nil), steps...)}
}

// Apply runs every step. jsonRoot may be nil when only env is in play.
// Mutations to jsonRoot are visible to later steps and to the caller.
func (r *Runner) Apply(env Env, jsonRoot map[string]any) ([]Deprecation, error) {
	if env == nil {
		env = OSEnv{}
	}
	var all []Deprecation
	for _, step := range r.steps {
		deps, err := step(env, jsonRoot)
		if err != nil {
			return all, err
		}
		all = append(all, deps...)
	}
	return all, nil
}

// Warn writes each deprecation to stderr once (boot nudge).
func Warn(deps []Deprecation) {
	for _, d := range deps {
		fmt.Fprintf(os.Stderr, "warning: %s\n", d.String())
	}
}

// LookupBool returns a parsed bool when key is set. Missing key → ok false.
func LookupBool(env Env, key string) (value bool, ok bool, err error) {
	raw, found := env.Lookup(key)
	if !found {
		return false, false, nil
	}
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, true, fmt.Errorf("%s: %w", key, err)
	}
	return v, true, nil
}

// LookupString returns the value when key is set and non-empty after trim.
func LookupString(env Env, key string) (value string, ok bool) {
	raw, found := env.Lookup(key)
	if !found {
		return "", false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	return raw, true
}

// Object returns a nested object under key, creating it if missing.
func Object(root map[string]any, key string) map[string]any {
	if root == nil {
		return nil
	}
	if v, ok := root[key]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	m := map[string]any{}
	root[key] = m
	return m
}

// HasKey reports whether key is present on m (including explicit null/false).
func HasKey(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}

// AsString coerces a JSON value to string.
func AsString(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case fmt.Stringer:
		return t.String(), true
	default:
		return "", false
	}
}

// AsBool coerces a JSON value to bool.
func AsBool(v any) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(t))
		if err != nil {
			return false, false
		}
		return b, true
	default:
		return false, false
	}
}
