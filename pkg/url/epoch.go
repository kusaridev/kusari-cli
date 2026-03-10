package url

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// hashRemote creates a short hash of the remote URL for use in sort keys.
// Returns first 8 characters of SHA256 hash.
func hashRemote(remote string) string {
	if remote == "" {
		return "local"
	}
	hash := sha256.Sum256([]byte(remote))
	return hex.EncodeToString(hash[:])[:8]
}

// CreateSortString creates a URL-encoded sort string from user ID, epoch, and repo metadata.
/* Sort Key format (NEW - includes repo metadata for incremental scanning):

cli-user: cli-user|{{remoteHash}}|{{dirName}}|{{branch}}|{{user sub}}|{{epoch}}
cli-user-full: cli-user-full|{{remoteHash}}|{{dirName}}|{{branch}}|{{user sub}}|{{epoch}}
cli-api: cli-api|{{remoteHash}}|{{dirName}}|{{branch}}|machine|{{epoch}}
cli-api-full: cli-api-full|{{remoteHash}}|{{dirName}}|{{branch}}|machine|{{epoch}}

Status entries append: |status|{{status}}
*/
func CreateSortString(userID, epoch string, full, isMachine bool, remote, dirName, branch string) string {
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

	remoteHash := hashRemote(remote)

	// New format: prefix|remoteHash|dirName|branch|userID|epoch
	sortString := fmt.Sprintf("%s|%s|%s|%s|%s|%s", prefix, remoteHash, dirName, branch, userID, epoch)
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
