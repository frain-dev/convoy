package models

type OnboardItem struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	EventType    string `json:"event_type"`
	AuthUsername string `json:"auth_username"`
	AuthPassword string `json:"auth_password"`
}

type BulkOnboardRequest struct {
	Items []OnboardItem `json:"items"`
}

type BulkOnboardAcceptedResponse struct {
	BatchCount int    `json:"batch_count"`
	TotalItems int    `json:"total_items"`
	Message    string `json:"message"`
}

type OnboardValidationError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type BulkOnboardDryRunResponse struct {
	TotalRows  int                      `json:"total_rows"`
	ValidCount int                      `json:"valid_count"`
	Errors     []OnboardValidationError `json:"errors"`
}
