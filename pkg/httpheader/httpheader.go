package httpheader

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
