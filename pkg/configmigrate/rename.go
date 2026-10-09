package configmigrate

import "strings"

// RenameEnvString copies old → new via set when new is unset and old is set.
func RenameEnvString(oldKey, newKey string, set func(string)) Step {
	return func(env Env, _ map[string]any) ([]Deprecation, error) {
		if _, ok := LookupString(env, newKey); ok {
			return nil, nil
		}
		v, ok := LookupString(env, oldKey)
		if !ok {
			return nil, nil
		}
		set(v)
		return []Deprecation{{Old: oldKey, New: newKey}}, nil
	}
}

// RenameEnvBool copies old → new via set when new is unset and old is set.
func RenameEnvBool(oldKey, newKey string, set func(bool)) Step {
	return func(env Env, _ map[string]any) ([]Deprecation, error) {
		if _, ok, err := LookupBool(env, newKey); err != nil {
			return nil, err
		} else if ok {
			return nil, nil
		}
		v, ok, err := LookupBool(env, oldKey)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		set(v)
		return []Deprecation{{Old: oldKey, New: newKey}}, nil
	}
}

// RenameJSONString moves a nested string from oldPath to newPath when the new
// leaf is absent. Paths are dotted object keys (e.g. retention_policy, policy).
func RenameJSONString(oldPath, newPath []string) Step {
	return func(_ Env, root map[string]any) ([]Deprecation, error) {
		if root == nil || len(oldPath) == 0 || len(newPath) == 0 {
			return nil, nil
		}
		if jsonLeafPresent(root, newPath) {
			return nil, nil
		}
		v, ok := jsonStringAt(root, oldPath)
		if !ok {
			return nil, nil
		}
		setJSONString(root, newPath, v)
		return []Deprecation{{Old: strings.Join(oldPath, "."), New: strings.Join(newPath, ".")}}, nil
	}
}

// RenameJSONBool moves a nested bool from oldPath to newPath when the new leaf
// is absent.
func RenameJSONBool(oldPath, newPath []string) Step {
	return func(_ Env, root map[string]any) ([]Deprecation, error) {
		if root == nil || len(oldPath) == 0 || len(newPath) == 0 {
			return nil, nil
		}
		if jsonLeafPresent(root, newPath) {
			return nil, nil
		}
		v, ok := jsonBoolAt(root, oldPath)
		if !ok {
			return nil, nil
		}
		setJSONBool(root, newPath, v)
		return []Deprecation{{Old: strings.Join(oldPath, "."), New: strings.Join(newPath, ".")}}, nil
	}
}

func jsonLeafPresent(root map[string]any, path []string) bool {
	m := root
	for i, key := range path {
		if i == len(path)-1 {
			return HasKey(m, key)
		}
		next, ok := m[key].(map[string]any)
		if !ok {
			return false
		}
		m = next
	}
	return false
}

func jsonStringAt(root map[string]any, path []string) (string, bool) {
	m := root
	for i, key := range path {
		if i == len(path)-1 {
			return AsString(m[key])
		}
		next, ok := m[key].(map[string]any)
		if !ok {
			return "", false
		}
		m = next
	}
	return "", false
}

func jsonBoolAt(root map[string]any, path []string) (bool, bool) {
	m := root
	for i, key := range path {
		if i == len(path)-1 {
			return AsBool(m[key])
		}
		next, ok := m[key].(map[string]any)
		if !ok {
			return false, false
		}
		m = next
	}
	return false, false
}

func setJSONString(root map[string]any, path []string, v string) {
	m := root
	for i, key := range path {
		if i == len(path)-1 {
			m[key] = v
			return
		}
		m = Object(m, key)
	}
}

func setJSONBool(root map[string]any, path []string, v bool) {
	m := root
	for i, key := range path {
		if i == len(path)-1 {
			m[key] = v
			return
		}
		m = Object(m, key)
	}
}
