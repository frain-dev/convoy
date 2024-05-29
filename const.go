package convoy

import "time"

const (
	HttpPost HttpMethod = "POST"
	HttpGet  HttpMethod = "GET"
)

const (
	HTTP_RATE_LIMIT         = 50
	HTTP_RATE_LIMIT_PER_MIN = HTTP_RATE_LIMIT * 60

	INGRESS_RATE_LIMIT         = 100
	INGRESS_RATE_LIMIT_PER_MIN = HTTP_RATE_LIMIT * 60

	EGRESS_RATE_LIMIT         = 100
	EGRESS_RATE_LIMIT_PER_MIN = HTTP_RATE_LIMIT * 60

	HTTP_TIMEOUT             = 10
	HTTP_TIMEOUT_IN_DURATION = time.Duration(HTTP_TIMEOUT) * time.Second
)
