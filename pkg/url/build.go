package url

import (
	"fmt"
	"net/url"
)

func Build(baseURL string, pathSegments ...string) (*string, error) {
	parsedURL, err := url.Parse(baseURL)
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to parse base URL: %w", err)
	}

	for _, segment := range pathSegments {
		parsedURL = parsedURL.JoinPath(segment)
	}
	fullUrl := parsedURL.String()
	return &fullUrl, nil
}
