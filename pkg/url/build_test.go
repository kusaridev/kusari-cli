package url

import (
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

func Test_GetIDsFromUrl_Success(t *testing.T) {
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/human/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=ASIAS7LCAOM53APYAJ26%2F20251021%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20251021T214101Z&X-Amz-Expires=300&X-Amz-Security-Token=asdf1234qwer4567&X-Amz-SignedHeaders=host&x-id=PutObject&X-Amz-Signature=aafacd4d8cd5c1a1aa405138b516c37db775698d75f4a798dbb8f0e6a6009378"

	workspaceID, userID, epoch, err := GetIDsFromUrl(presignUrl)

	assert.Nil(t, err)
	assert.Equal(t, "4382f4d8-3a11-401f-a9ba-3b1702f6917e", workspaceID)
	assert.Equal(t, "6a5404db-a484-4115-8a69-a9def45a8fe3", userID)
	assert.Equal(t, "1761082861", epoch)
}

func Test_GetIDsFromUrl_NoQueryParams(t *testing.T) {
	presignUrl := "https://example.com/workspace/test-workspace-id/user/human/test-user-id/diff/blob/123"

	workspaceID, userID, epoch, err := GetIDsFromUrl(presignUrl)

	assert.Nil(t, err)
	assert.Equal(t, "test-workspace-id", workspaceID)
	assert.Equal(t, "test-user-id", userID)
	assert.Equal(t, "123", epoch)
}

func Test_GetIDsFromUrl_MissingWorkspace(t *testing.T) {
	presignUrl := "https://example.com/user/human/test-user-id/diff/blob/123"

	_, _, _, err := GetIDsFromUrl(presignUrl)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "workspace ID not found")
}

func Test_GetIDsFromUrl_MissingUser(t *testing.T) {
	presignUrl := "https://example.com/workspace/test-workspace-id/diff/blob/123"

	_, _, _, err := GetIDsFromUrl(presignUrl)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "user ID not found")
}

func Test_GetIDsFromUrl_InvalidURL(t *testing.T) {
	presignUrl := "://invalid-url"

	_, _, _, err := GetIDsFromUrl(presignUrl)

	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "error parsing URL")
}

func Test_CreateSortString_Basic(t *testing.T) {
	userID := "6a5404db-a484-4115-8a69-a9def45a8fe3"
	epoch := "1761082861"

	result := CreateSortString(userID, epoch, false)

	assert.Equal(t, "cli-user%7C6a5404db-a484-4115-8a69-a9def45a8fe3%7C1761082861", result)
}

func Test_CreateSortString_Full(t *testing.T) {
	userID := "6a5404db-a484-4115-8a69-a9def45a8fe3"
	epoch := "1761082861"

	result := CreateSortString(userID, epoch, true)

	assert.Equal(t, "cli-user-full%7C6a5404db-a484-4115-8a69-a9def45a8fe3%7C1761082861", result)
}

func Test_CreateSortString_WithSpecialChars(t *testing.T) {
	userID := "user@example.com"
	epoch := "2024/10/21"

	result := CreateSortString(userID, epoch, false)

	assert.Equal(t, "cli-user%7Cuser%40example.com%7C2024%2F10%2F21", result)
}

func Test_CreateSortString_EmptyValues(t *testing.T) {
	result := CreateSortString("", "", false)

	assert.Equal(t, "cli-user%7C%7C", result)
}

func Test_CreateSortString_Integration(t *testing.T) {
	// Test that we can use the output from GetIDsFromUrl
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/human/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256"

	_, userID, epoch, err := GetIDsFromUrl(presignUrl)
	assert.Nil(t, err)

	result := CreateSortString(userID, epoch, false)

	assert.Equal(t, "cli-user%7C6a5404db-a484-4115-8a69-a9def45a8fe3%7C1761082861", result)
}

func Test_CreateSortString_Integration_Full(t *testing.T) {
	// Test that we can use the output from GetIDsFromUrl with full=true
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/human/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256"

	_, userID, epoch, err := GetIDsFromUrl(presignUrl)
	assert.Nil(t, err)

	result := CreateSortString(userID, epoch, true)

	assert.Equal(t, "cli-user-full%7C6a5404db-a484-4115-8a69-a9def45a8fe3%7C1761082861", result)
}
