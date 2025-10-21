package url

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// CreateSortString creates a URL-encoded sort string from user ID and epoch.
// Format: "cli-user|{userID}|{epoch}" or "cli-user-full|{userID}|{epoch}" (URL-encoded)
func CreateSortString(userID string, epoch string, full bool) string {
	prefix := "cli-user"
	if full {
		prefix = "cli-user-full"
	}
	sortString := fmt.Sprintf("%s|%s|%s", prefix, userID, epoch)
	return url.QueryEscape(sortString)
}

// GetIDsFromUrl extracts the workspace ID, user ID, and epoch from a presigned URL.
// Returns workspaceID, userID, epoch, error.
// Expected URL format: .../workspace/{workspaceID}/user/{userType}/{userID}/.../{epoch}
func GetIDsFromUrl(presignUrl string) (string, string, string, error) {
	u, err := url.Parse(presignUrl)
	if err != nil {
		return "", "", "", fmt.Errorf("error parsing URL: %w", err)
	}

	// Split the path into segments
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")

	var workspaceID, userID string

	// Find workspace ID
	for i, segment := range segments {
		if segment == "workspace" && i+1 < len(segments) {
			workspaceID = segments[i+1]
		}
		if segment == "user" && i+2 < len(segments) {
			// Skip userType (segments[i+1]) and get userID (segments[i+2])
			userID = segments[i+2]
		}
	}

	// Get epoch from the last segment
	epoch := path.Base(u.Path)

	if workspaceID == "" {
		return "", "", "", fmt.Errorf("workspace ID not found in URL")
	}
	if userID == "" {
		return "", "", "", fmt.Errorf("user ID not found in URL")
	}
	if epoch == "" || epoch == "/" {
		return "", "", "", fmt.Errorf("epoch not found in URL")
	}

	return workspaceID, userID, epoch, nil
}
