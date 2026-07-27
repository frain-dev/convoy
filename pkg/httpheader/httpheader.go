package httpheader

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

// HTTPHeader is our custom  header type that can merge fields.
type HTTPHeader map[string][]string

// MergeHeaders is used to merge two headers together as one.
// It takes all the incoming header values and merges them to HTTPHeader
// without replacing the previous value. Existing keys win case-insensitively
// so protected headers such as Authorization cannot be duplicated under a
// different casing by attacker-controlled event headers.
func (h HTTPHeader) MergeHeaders(nh HTTPHeader) {
	for k, v := range nh {
		if headerKeyExists(h, k) {
			continue
		}

		h[k] = v
	}
}

func headerKeyExists(h HTTPHeader, key string) bool {
	if _, ok := h[key]; ok {
		return true
	}
	for existing := range h {
		if strings.EqualFold(existing, key) {
			return true
		}
	}
	return false
}

func (h *HTTPHeader) Scan(value interface{}) error {
	if value == nil {
		*h = nil
		return nil
	}

	b, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("unsupported value type %T", value)
	}

	if string(b) == "null" {
		return nil
	}

	if err := json.Unmarshal(b, &h); err != nil {
		return err
	}

	return nil
}

func (h HTTPHeader) Value() (driver.Value, error) {
	if h == nil {
		return nil, nil
	}

	b, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}

	return b, nil
}
