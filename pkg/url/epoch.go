package url

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"path"
	"strings"
)

// GetIDsFromUrl reads the uploader and epoch from a presigned upload URL of the
// form .../user/{human|machine}/{userID}/.../{epoch}.
func GetIDsFromUrl(presignUrl string) (userID, epoch string, isMachine bool, err error) {
	u, err := url.Parse(presignUrl)
	if err != nil {
		return "", "", false, fmt.Errorf("error parsing URL: %w", err)
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")

	var userType string
	for i, segment := range segments {
		if segment == "user" && i+2 < len(segments) {
			userType = segments[i+1]
			userID = segments[i+2]
		}
	}

	epoch = path.Base(u.Path)

	if userID == "" {
		return "", "", false, fmt.Errorf("user ID not found in URL")
	}
	if epoch == "" || epoch == "/" {
		return "", "", false, fmt.Errorf("epoch not found in URL")
	}

	return userID, epoch, userType == "machine", nil
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
