package url

import (
	"fmt"
	"net/url"
	"path"
)

func GetEpochFromUrl(presignUrl string) (*string, error) {
	u, err := url.Parse(presignUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}
	epoch := path.Base(u.Path)
	return &epoch, nil
}
