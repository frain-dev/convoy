package url

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

var (
	ErrURLTemplateNotEnabled      = errors.New("endpoint URL templates are not enabled")
	ErrURLTemplateInvalidToken    = errors.New("endpoint URL template contains an invalid token")
	ErrURLTemplateUnsupportedPart = errors.New("endpoint URL templates are only supported in the path or query")
	ErrURLTemplateInvalidURL      = errors.New("endpoint url must include a valid host")
)

var templateTokenPattern = regexp.MustCompile(`^\{[A-Za-z_][A-Za-z0-9_]*\}`)

type rawURLParts struct {
	scheme string
	host   string
	path   string
	query  string
}

func ContainsTemplate(rawURL string) bool {
	parts := splitRawURL(rawURL)
	return containsValidToken(parts.path) || containsValidToken(parts.query)
}

func ValidateEndpointTemplate(rawURL string, allowTemplates bool) (*url.URL, bool, error) {
	parts := splitRawURL(rawURL)
	hasRawBraces := strings.ContainsAny(parts.scheme, "{}") ||
		strings.ContainsAny(parts.host, "{}") ||
		strings.ContainsAny(parts.path, "{}") ||
		strings.ContainsAny(parts.query, "{}")
	hasTemplate := containsValidToken(parts.path) || containsValidToken(parts.query)

	if hasRawBraces && !allowTemplates {
		return nil, false, ErrURLTemplateNotEnabled
	}

	if strings.ContainsAny(parts.scheme, "{}") || strings.ContainsAny(parts.host, "{}") {
		return nil, false, ErrURLTemplateUnsupportedPart
	}

	if hasRawBraces {
		if err := validateTemplateTokens(parts.path); err != nil {
			return nil, false, err
		}
		if err := validateTemplateTokens(parts.query); err != nil {
			return nil, false, err
		}
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, false, err
	}
	if parsedURL.Host == "" {
		return nil, false, ErrURLTemplateInvalidURL
	}

	return parsedURL, hasTemplate, nil
}

func TemplateMatches(templateURL, concreteURL string) (bool, error) {
	templateParts := splitRawURL(templateURL)
	concreteParts := splitRawURL(concreteURL)

	if strings.ContainsAny(concreteParts.scheme, "{}") ||
		strings.ContainsAny(concreteParts.host, "{}") ||
		strings.ContainsAny(concreteParts.path, "{}") ||
		strings.ContainsAny(concreteParts.query, "{}") {
		return false, nil
	}

	if _, hasTemplate, err := ValidateEndpointTemplate(templateURL, true); err != nil || !hasTemplate {
		return false, err
	}

	if templateParts.scheme != concreteParts.scheme || templateParts.host != concreteParts.host {
		return false, nil
	}

	pathMatches, err := templatePartMatches(
		normalizeTemplateMatchPath(templateParts.path),
		normalizeTemplateMatchPath(concreteParts.path),
		`[^/?#]+`,
	)
	if err != nil || !pathMatches {
		return pathMatches, err
	}

	return templateQueryMatches(templateParts.query, concreteParts.query)
}

func splitRawURL(rawURL string) rawURLParts {
	var parts rawURLParts

	schemeEnd := strings.Index(rawURL, "://")
	if schemeEnd >= 0 {
		parts.scheme = rawURL[:schemeEnd]
		rawURL = rawURL[schemeEnd+3:]
	}

	hostEnd := len(rawURL)
	for _, sep := range []string{"/", "?", "#"} {
		if idx := strings.Index(rawURL, sep); idx >= 0 && idx < hostEnd {
			hostEnd = idx
		}
	}

	parts.host = rawURL[:hostEnd]
	remainder := rawURL[hostEnd:]
	if hashIdx := strings.Index(remainder, "#"); hashIdx >= 0 {
		remainder = remainder[:hashIdx]
	}

	if queryIdx := strings.Index(remainder, "?"); queryIdx >= 0 {
		parts.path = remainder[:queryIdx]
		parts.query = remainder[queryIdx+1:]
		return parts
	}

	parts.path = remainder
	return parts
}

func normalizeTemplateMatchPath(path string) string {
	if path == "" {
		return "/"
	}
	if len(path) > 1 {
		return strings.TrimRight(path, "/")
	}
	return path
}

func containsValidToken(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		return templateTokenPattern.MatchString(s[i:])
	}

	return false
}

func validateTemplateTokens(s string) error {
	for i := 0; i < len(s); {
		switch s[i] {
		case '{':
			token := templateTokenPattern.FindString(s[i:])
			if token == "" {
				return ErrURLTemplateInvalidToken
			}
			i += len(token)
		case '}':
			return ErrURLTemplateInvalidToken
		default:
			i++
		}
	}

	return nil
}

func templatePartMatches(templatePart, concretePart, replacement string) (bool, error) {
	pattern := strings.Builder{}
	pattern.WriteString("^")

	for i := 0; i < len(templatePart); {
		if templatePart[i] != '{' {
			pattern.WriteString(regexp.QuoteMeta(string(templatePart[i])))
			i++
			continue
		}

		token := templateTokenPattern.FindString(templatePart[i:])
		if token == "" {
			return false, ErrURLTemplateInvalidToken
		}

		pattern.WriteString(replacement)
		i += len(token)
	}

	pattern.WriteString("$")
	return regexp.MatchString(pattern.String(), concretePart)
}

func templateQueryMatches(templateQuery, concreteQuery string) (bool, error) {
	if templateQuery == "" {
		return true, nil
	}

	concretePairs := strings.Split(concreteQuery, "&")
	for _, templatePair := range strings.Split(templateQuery, "&") {
		if templatePair == "" {
			continue
		}

		pairMatched := false
		for _, concretePair := range concretePairs {
			matched, err := templatePartMatches(templatePair, concretePair, `[^&#]+`)
			if err != nil {
				return false, err
			}
			if matched {
				pairMatched = true
				break
			}
		}

		if !pairMatched {
			return false, nil
		}
	}

	return true, nil
}
