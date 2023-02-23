package httpheader

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// HTTPHeader is our custom  header type that can merge fields.
type HTTPHeader map[string][]string

// MergeHeaders is used to merge two headers together as one.
// It takes all the incoming header values and merges them to HTTPHeader
// without replacing the previous value
func (h HTTPHeader) MergeHeaders(nh HTTPHeader) {
	for k, v := range nh {
		if _, ok := h[k]; ok {
			continue
		}

		h[k] = v
	}
}

func (h *HTTPHeader) Scan(value interface{}) error {
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
