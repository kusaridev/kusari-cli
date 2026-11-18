package url

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"strings"
)

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

// ScanEventKey is workspace-api's event_key for a CLI scan: the hex SHA-256 of
// {type}|{uploader}|{epoch}. An API key's uploader is its own subject, not
// "machine".
func ScanEventKey(userID, epoch string, full, isMachine bool) string {
	hash := sha256.Sum256([]byte(cliScanType(full, isMachine) + "|" + userID + "|" + epoch))
	return hex.EncodeToString(hash[:])
}

func cliScanType(full, isMachine bool) string {
	scanType := "cli-user"
	if isMachine {
		scanType = "cli-api"
	}
	if full {
		scanType += "-full"
	}
	return scanType
}
