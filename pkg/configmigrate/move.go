package configmigrate

import "strings"

// MoveJSONObjectField copies one field from src object path to dst object path
// when the destination leaf is absent. srcPath and dstPath are full leaf paths.
// Prefer RenameJSONString / RenameJSONBool for typed moves; this is for
// arbitrary JSON values (nested objects, numbers, etc.).
func MoveJSONValue(oldPath, newPath []string) Step {
	return func(_ Env, root map[string]any) ([]Deprecation, error) {
		if root == nil || len(oldPath) == 0 || len(newPath) == 0 {
			return nil, nil
		}
		if jsonLeafPresent(root, newPath) {
			return nil, nil
		}
		v, ok := jsonValueAt(root, oldPath)
		if !ok {
			return nil, nil
		}
		setJSONValue(root, newPath, v)
		return []Deprecation{{Old: strings.Join(oldPath, "."), New: strings.Join(newPath, ".")}}, nil
	}
}

func jsonValueAt(root map[string]any, path []string) (any, bool) {
	m := root
	for i, key := range path {
		if i == len(path)-1 {
			v, ok := m[key]
			return v, ok
		}
		next, ok := m[key].(map[string]any)
		if !ok {
			return nil, false
		}
		m = next
	}
	return nil, false
}

func setJSONValue(root map[string]any, path []string, v any) {
	m := root
	for i, key := range path {
		if i == len(path)-1 {
			m[key] = v
			return
		}
		m = Object(m, key)
	}
}
