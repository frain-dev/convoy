package url

import "net/url"

func ConcatQueryParams(targetURL, query string) (string, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return "", err
	}

	parsedValues, err := url.ParseQuery(query)
	if err != nil {
		return "", err
	}

	q := u.Query()

	for k, v := range parsedValues {
		for _, s := range v {
			q.Add(k, s)
		}
	}

	u.RawQuery = q.Encode()

	return u.String(), nil
}
