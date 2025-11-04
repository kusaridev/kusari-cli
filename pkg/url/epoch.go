package url

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// CreateSortString creates a URL-encoded sort string from user ID and epoch.
/* Sort Key format:

cli-user: cli-user|{{user sub}}|{{epoch}}
cli-user-full: cli-user-full|{{user sub}}|{{epoch}}
cli-api: cli-api|machine|{{epoch}}
cli-api-full: cli-api-full|machine|{{epoch}}
cli-user: cli-user|{{user sub}}|{{timestamp}}|status|{{status}}
cli-user-full: cli-user-full|{{user sub}}|{{timestamp}}|status|{{status}}
cli-api: cli-api|machine|{{timestamp}}|status|{{status}}
cli-api-full: cli-api-full|machine|{{timestamp}}|status|{{status}}
*/
func CreateSortString(userID string, epoch string, full, isMachine bool) string {
	var prefix string

	if !isMachine {
		prefix = "cli-user"
		if full {
			prefix = "cli-user-full"
		}
	} else {
		prefix = "cli-api"
		if full {
			prefix = "cli-api-full"
		}
		userID = "machine"
	}

	sortString := fmt.Sprintf("%s|%s|%s", prefix, userID, epoch)
	return url.QueryEscape(sortString)
}

// GetIDsFromUrl extracts the workspace ID, user ID, and epoch from a presigned URL.
// Returns workspaceID, userID, epoch, error.
// Expected URL format: .../workspace/{workspaceID}/user/{userType}/{userID}/.../{epoch}
func GetIDsFromUrl(presignUrl string) (string, string, string, bool, error) {
	u, err := url.Parse(presignUrl)
	if err != nil {
		return "", "", "", false, fmt.Errorf("error parsing URL: %w", err)
	}

	// Split the path into segments
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")

	var workspaceID, userID, isMachineStr string
	var isMachine bool

	// Find workspace ID
	for i, segment := range segments {
		if segment == "workspace" && i+1 < len(segments) {
			workspaceID = segments[i+1]
		}
		if segment == "user" && i+2 < len(segments) {
			// Skip userType (segments[i+1]) and get userID (segments[i+2])
			userID = segments[i+2]
			isMachineStr = segments[i+1]
		}
	}

	// Get epoch from the last segment
	epoch := path.Base(u.Path)

	if workspaceID == "" {
		return "", "", "", false, fmt.Errorf("workspace ID not found in URL")
	}
	if userID == "" {
		return "", "", "", false, fmt.Errorf("user ID not found in URL")
	}
	if epoch == "" || epoch == "/" {
		return "", "", "", false, fmt.Errorf("epoch not found in URL")
	}
	if isMachineStr == "machine" {
		isMachine = true
	}

	return workspaceID, userID, epoch, isMachine, nil
}
