package constants

// Content type constants for endpoints
const (
	ContentTypeJSON           = "application/json"
	ContentTypeFormURLEncoded = "application/x-www-form-urlencoded"
)

// ValidContentTypes returns a slice of all valid content types
func ValidContentTypes() []string {
	return []string{
		ContentTypeJSON,
		ContentTypeFormURLEncoded,
	}
}

// IsValidContentType checks if the given content type is valid
func IsValidContentType(contentType string) bool {
	for _, validType := range ValidContentTypes() {
		if contentType == validType {
			return true
		}
	}
	return false
}
