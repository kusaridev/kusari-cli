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

func Test_GetIDsFromUrl_Success_Machine(t *testing.T) {
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/machine/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=ASIAS7LCAOM53APYAJ26%2F20251021%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20251021T214101Z&X-Amz-Expires=300&X-Amz-Security-Token=asdf1234qwer4567&X-Amz-SignedHeaders=host&x-id=PutObject&X-Amz-Signature=aafacd4d8cd5c1a1aa405138b516c37db775698d75f4a798dbb8f0e6a6009378"

	userID, epoch, isMachine, err := GetIDsFromUrl(presignUrl)

	assert.Nil(t, err)
	assert.Equal(t, "6a5404db-a484-4115-8a69-a9def45a8fe3", userID)
	assert.True(t, isMachine)
	assert.Equal(t, "1761082861", epoch)
}

func Test_GetIDsFromUrl_Success_Human(t *testing.T) {
	presignUrl := "https://inspector-bundle-upload-dev-us-east-1.s3.us-east-1.amazonaws.com/workspace/4382f4d8-3a11-401f-a9ba-3b1702f6917e/user/human/6a5404db-a484-4115-8a69-a9def45a8fe3/diff/blob/1761082861?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=ASIAS7LCAOM53APYAJ26%2F20251021%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-Date=20251021T214101Z&X-Amz-Expires=300&X-Amz-Security-Token=asdf1234qwer4567&X-Amz-SignedHeaders=host&x-id=PutObject&X-Amz-Signature=aafacd4d8cd5c1a1aa405138b516c37db775698d75f4a798dbb8f0e6a6009378"

	userID, epoch, isMachine, err := GetIDsFromUrl(presignUrl)

	assert.Nil(t, err)
	assert.Equal(t, "6a5404db-a484-4115-8a69-a9def45a8fe3", userID)
	assert.False(t, isMachine)
	assert.Equal(t, "1761082861", epoch)
}

func Test_GetIDsFromUrl_NoQueryParams(t *testing.T) {
	presignUrl := "https://example.com/workspace/test-workspace-id/user/human/test-user-id/diff/blob/123"

	userID, epoch, isMachine, err := GetIDsFromUrl(presignUrl)

	assert.Nil(t, err)
	assert.Equal(t, "test-user-id", userID)
	assert.False(t, isMachine)
	assert.Equal(t, "123", epoch)
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

// The hashes are sha256 of {type}|{uploader}|{epoch}, which is what workspace-api
// stores as a CLI scan's event_key. The API-key row catches collapsing the
// uploader to "machine", and hashing the subject while it is still URL-encoded.
func Test_ScanEventKey(t *testing.T) {
	tests := []struct {
		name       string
		presignUrl string
		full       bool
		want       string
	}{
		{
			name:       "user diff scan",
			presignUrl: "https://example.com/workspace/ws/user/human/dev%2Bci%40example.com/diff/blob/1761082861?X-Amz-Signature=abc",
			want:       "dd9dcf69cd89b292f85e057cc26893a6f3b8a8986572706f61e81f18b01eaaaf",
		},
		{
			name:       "API key risk check",
			presignUrl: "https://example.com/workspace/ws/user/machine/spiffe%3Atd%3Akusari.dev%3Ans%3Aci%3Asa%3Arunner/full/blob/1761082861?X-Amz-Signature=abc",
			full:       true,
			want:       "02a6c4d59c3d5337da80fdf5f4f79b42cb871a0688d6c5aab3fa66c7b8bf9cf2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, epoch, isMachine, err := GetIDsFromUrl(tt.presignUrl)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, ScanEventKey(userID, epoch, tt.full, isMachine))
		})
	}
}
