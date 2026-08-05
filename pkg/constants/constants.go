// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package constants

const (
	// DefaultPlatformURL is the default Kusari platform API URL
	DefaultPlatformURL = "https://platform.api.us.kusari.cloud/"

	// DefaultConsoleURL is the default Kusari console URL
	DefaultConsoleURL = "https://console.us.kusari.cloud/"

	// DefaultAuthURL is the default Kusari authentication endpoint URL
	DefaultAuthURL = "https://auth.us.kusari.cloud/"
)

// Upload endpoint constants, used only by `kusari connectivity check`.
//
// Uploads never derive a URL from these: the real host is issued per-request by
// the platform presign response (pkg/repo/upload.go). They are a second source
// of truth, so update them if platform bucket naming changes.
const (
	// InspectorUploadHostPattern takes env, s3 region, s3 region.
	InspectorUploadHostPattern = "inspector-bundle-upload-%s-%s.s3.%s.amazonaws.com"

	DefaultUploadEnv = "prod"

	// DefaultS3Region: non-us Kusari regions need a real region mapping here,
	// not string interpolation from the region name.
	DefaultS3Region = "us-east-1"
)
