package url

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Build_Base(t *testing.T) {
	actual, e := Build("https://jerry.wilson")

	assert.Nil(t, e)
	assert.Equal(t, "https://jerry.wilson", *actual)
}

func Test_Build_Hostname_Error(t *testing.T) {
	_, e := Build("*****")

	assert.NotNil(t, e)
}

func Test_Build_Base_Trailing_Slash(t *testing.T) {
	actual, e := Build("https://jerry.wilson/")

	assert.Nil(t, e)
	assert.Equal(t, "https://jerry.wilson/", *actual)
}

func Test_Build_Path(t *testing.T) {
	actual, e := Build("https://jerry.wilson", "a", "b")

	assert.Nil(t, e)
	assert.Equal(t, "https://jerry.wilson/a/b", *actual)
}

func Test_Build_Path_Trailing_Slash(t *testing.T) {
	actual, e := Build("https://jerry.wilson/", "a", "b")

	assert.Nil(t, e)
	assert.Equal(t, "https://jerry.wilson/a/b", *actual)
}

func Test_GetIDsFromUrl_Success_Machine(t *testing.T) {
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/machine/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=ASIAS7LCAOM53APYAJ26%2F20251021%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20251021T214101Z&X-Amz-Expires=300&X-Amz-Security-Token=asdf1234qwer4567&X-Amz-SignedHeaders=host&x-id=PutObject&X-Amz-Signature=aafacd4d8cd5c1a1aa405138b516c37db775698d75f4a798dbb8f0e6a6009378"

	workspaceID, userID, epoch, isMachine, err := GetIDsFromUrl(presignUrl)

	assert.Nil(t, err)
	assert.Equal(t, "4382f4d8-3a11-401f-a9ba-3b1702f6917e", workspaceID)
	assert.Equal(t, "6a5404db-a484-4115-8a69-a9def45a8fe3", userID)
	assert.True(t, isMachine)
	assert.Equal(t, "1761082861", epoch)
}

func Test_GetIDsFromUrl_Success_Human(t *testing.T) {
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/human/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=ASIAS7LCAOM53APYAJ26%2F20251021%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20251021T214101Z&X-Amz-Expires=300&X-Amz-Security-Token=asdf1234qwer4567&X-Amz-SignedHeaders=host&x-id=PutObject&X-Amz-Signature=aafacd4d8cd5c1a1aa405138b516c37db775698d75f4a798dbb8f0e6a6009378"

	workspaceID, userID, epoch, isMachine, err := GetIDsFromUrl(presignUrl)

	assert.Nil(t, err)
	assert.Equal(t, "4382f4d8-3a11-401f-a9ba-3b1702f6917e", workspaceID)
	assert.Equal(t, "6a5404db-a484-4115-8a69-a9def45a8fe3", userID)
	assert.False(t, isMachine)
	assert.Equal(t, "1761082861", epoch)
}

func Test_GetIDsFromUrl_NoQueryParams(t *testing.T) {
	presignUrl := "https://example.com/workspace/test-workspace-id/user/human/test-user-id/diff/blob/123"

	workspaceID, userID, epoch, isMachine, err := GetIDsFromUrl(presignUrl)

	assert.Nil(t, err)
	assert.Equal(t, "test-workspace-id", workspaceID)
	assert.Equal(t, "test-user-id", userID)
	assert.False(t, isMachine)
	assert.Equal(t, "123", epoch)
}

func Test_GetIDsFromUrl_MissingWorkspace(t *testing.T) {
	presignUrl := "https://example.com/user/human/test-user-id/diff/blob/123"

	_, _, _, _, err := GetIDsFromUrl(presignUrl)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "workspace ID not found")
}

func Test_GetIDsFromUrl_MissingUser(t *testing.T) {
	presignUrl := "https://example.com/workspace/test-workspace-id/diff/blob/123"

	_, _, _, _, err := GetIDsFromUrl(presignUrl)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "user ID not found")
}

func Test_GetIDsFromUrl_InvalidURL(t *testing.T) {
	presignUrl := "://invalid-url"

	_, _, _, _, err := GetIDsFromUrl(presignUrl)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "error parsing URL")
}

// CreateSortString tests for cli-user (human users)
// New format: cli-user|{remoteHash}|{dirName}|{branch}|{userID}|{epoch}
func Test_CreateSortString_Basic_Human(t *testing.T) {
	userID := "6a5404db-a484-4115-8a69-a9def45a8fe3"
	epoch := "1761082861"
	remote := "https://github.com/example/repo.git"
	dirName := "myproject"
	branch := "main"

	result := CreateSortString(userID, epoch, false, false, remote, dirName, branch)

	// remoteHash is first 8 chars of SHA256 of remote
	// Expected format: cli-user|{remoteHash}|{dirName}|{branch}|{userID}|{epoch}
	assert.Contains(t, result, "cli-user%7C")
	assert.Contains(t, result, "%7Cmyproject%7Cmain%7C6a5404db-a484-4115-8a69-a9def45a8fe3%7C1761082861")
}

func Test_CreateSortString_Full_Human(t *testing.T) {
	userID := "6a5404db-a484-4115-8a69-a9def45a8fe3"
	epoch := "1761082861"
	remote := "https://github.com/example/repo.git"
	dirName := "myproject"
	branch := "main"

	result := CreateSortString(userID, epoch, true, false, remote, dirName, branch)

	assert.Contains(t, result, "cli-user-full%7C")
	assert.Contains(t, result, "%7Cmyproject%7Cmain%7C6a5404db-a484-4115-8a69-a9def45a8fe3%7C1761082861")
}

// CreateSortString tests for cli-api (machine users)
func Test_CreateSortString_Basic_Machine(t *testing.T) {
	userID := "6a5404db-a484-4115-8a69-a9def45a8fe3"
	epoch := "1761082861"
	remote := "https://github.com/example/repo.git"
	dirName := "myproject"
	branch := "main"

	result := CreateSortString(userID, epoch, false, true, remote, dirName, branch)

	assert.Contains(t, result, "cli-api%7C")
	assert.Contains(t, result, "%7Cmyproject%7Cmain%7Cmachine%7C1761082861")
}

func Test_CreateSortString_Full_Machine(t *testing.T) {
	userID := "6a5404db-a484-4115-8a69-a9def45a8fe3"
	epoch := "1761082861"
	remote := "https://github.com/example/repo.git"
	dirName := "myproject"
	branch := "main"

	result := CreateSortString(userID, epoch, true, true, remote, dirName, branch)

	assert.Contains(t, result, "cli-api-full%7C")
	assert.Contains(t, result, "%7Cmyproject%7Cmain%7Cmachine%7C1761082861")
}

func Test_CreateSortString_WithSpecialChars(t *testing.T) {
	userID := "user@example.com"
	epoch := "2024/10/21"
	remote := "https://github.com/example/repo.git"
	dirName := "my-project"
	branch := "feature/test"

	result := CreateSortString(userID, epoch, false, false, remote, dirName, branch)

	assert.Contains(t, result, "cli-user%7C")
	// Special chars in userID and epoch should be URL-encoded
	assert.Contains(t, result, "user%40example.com%7C2024%2F10%2F21")
}

func Test_CreateSortString_EmptyRemote(t *testing.T) {
	// When remote is empty (local repo), should use "local" as hash
	result := CreateSortString("user1", "123", false, false, "", "mydir", "main")

	assert.Contains(t, result, "cli-user%7Clocal%7Cmydir%7Cmain%7Cuser1%7C123")
}

func Test_CreateSortString_EmptyValues_Machine(t *testing.T) {
	result := CreateSortString("", "", false, true, "", "", "")

	assert.Contains(t, result, "cli-api%7Clocal%7C%7C%7Cmachine%7C")
}

// Integration tests
func Test_CreateSortString_Integration_Human(t *testing.T) {
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/human/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256"

	_, userID, epoch, isMachine, err := GetIDsFromUrl(presignUrl)
	assert.Nil(t, err)

	// Simulated repo metadata
	remote := "https://github.com/example/repo.git"
	dirName := "myproject"
	branch := "main"

	result := CreateSortString(userID, epoch, false, isMachine, remote, dirName, branch)

	assert.Contains(t, result, "cli-user%7C")
	assert.Contains(t, result, "%7C6a5404db-a484-4115-8a69-a9def45a8fe3%7C1761082861")
}

func Test_CreateSortString_Integration_Human_Full(t *testing.T) {
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/human/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256"

	_, userID, epoch, isMachine, err := GetIDsFromUrl(presignUrl)
	assert.Nil(t, err)

	remote := "https://github.com/example/repo.git"
	dirName := "myproject"
	branch := "main"

	result := CreateSortString(userID, epoch, true, isMachine, remote, dirName, branch)

	assert.Contains(t, result, "cli-user-full%7C")
	assert.Contains(t, result, "%7C6a5404db-a484-4115-8a69-a9def45a8fe3%7C1761082861")
}

func Test_CreateSortString_Integration_Machine(t *testing.T) {
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/machine/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256"

	_, userID, epoch, isMachine, err := GetIDsFromUrl(presignUrl)
	assert.Nil(t, err)
	assert.True(t, isMachine)

	remote := "https://github.com/example/repo.git"
	dirName := "myproject"
	branch := "main"

	result := CreateSortString(userID, epoch, false, isMachine, remote, dirName, branch)

	assert.Contains(t, result, "cli-api%7C")
	assert.Contains(t, result, "%7Cmachine%7C1761082861")
}

func Test_CreateSortString_Integration_Machine_Full(t *testing.T) {
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/machine/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256"

	_, userID, epoch, isMachine, err := GetIDsFromUrl(presignUrl)
	assert.Nil(t, err)
	assert.True(t, isMachine)

	remote := "https://github.com/example/repo.git"
	dirName := "myproject"
	branch := "main"

	result := CreateSortString(userID, epoch, true, isMachine, remote, dirName, branch)

	assert.Contains(t, result, "cli-api-full%7C")
	assert.Contains(t, result, "%7Cmachine%7C1761082861")
}

// Test hashRemote function through CreateSortString
func Test_CreateSortString_SameRemote_SameHash(t *testing.T) {
	remote := "https://github.com/example/repo.git"
	result1 := CreateSortString("user1", "123", false, false, remote, "dir1", "main")
	result2 := CreateSortString("user2", "456", false, false, remote, "dir1", "main")

	// Both should have the same remoteHash (8 chars after cli-user|)
	// Extract the hash portion (between first | and second |)
	parts1 := strings.Split(result1, "%7C")
	parts2 := strings.Split(result2, "%7C")

	assert.Equal(t, parts1[1], parts2[1], "Same remote should produce same hash")
}

func Test_CreateSortString_DifferentRemote_DifferentHash(t *testing.T) {
	result1 := CreateSortString("user1", "123", false, false, "https://github.com/org1/repo.git", "dir", "main")
	result2 := CreateSortString("user1", "123", false, false, "https://github.com/org2/repo.git", "dir", "main")

	parts1 := strings.Split(result1, "%7C")
	parts2 := strings.Split(result2, "%7C")

	assert.NotEqual(t, parts1[1], parts2[1], "Different remotes should produce different hashes")
}
