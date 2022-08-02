package convoy

const (
	HttpPost HttpMethod = "POST"
	HttpGet  HttpMethod = "GET"
)

const (
	RATE_LIMIT          = 5000
	RATE_LIMIT_DURATION = "1m"
	HTTP_TIMEOUT        = "30s"
)
