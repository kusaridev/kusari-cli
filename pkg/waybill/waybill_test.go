// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package waybill

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeTarGz builds an in-memory .tar.gz from a name → content map.
func makeTarGz(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range entries {
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o755,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

func TestDownloadAndVerify_Happy(t *testing.T) {
	payload := []byte("hello waybill")
	sum := sha256.Sum256(payload)
	wantHex := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	path, err := downloadAndVerify(context.Background(), srv.URL, wantHex)
	require.NoError(t, err)
	defer func() { _ = os.Remove(path) }()

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestDownloadAndVerify_HashMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("evil payload"))
	}))
	defer srv.Close()

	// Any hash that doesn't match the payload's SHA256 should be rejected.
	_, err := downloadAndVerify(context.Background(), srv.URL,
		"0000000000000000000000000000000000000000000000000000000000000000")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestDownloadAndVerify_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := downloadAndVerify(context.Background(), srv.URL, "doesntmatter")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}

func TestDownloadAndVerify_CaseInsensitiveHash(t *testing.T) {
	payload := []byte("case test")
	sum := sha256.Sum256(payload)
	wantHex := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	// SHA256SUMS files conventionally use lowercase, but accept upper too.
	path, err := downloadAndVerify(context.Background(), srv.URL, bytesToUpper(wantHex))
	require.NoError(t, err)
	_ = os.Remove(path)
}

func bytesToUpper(s string) string {
	out := make([]byte, len(s))
	for i := range s {
		if s[i] >= 'a' && s[i] <= 'z' {
			out[i] = s[i] - 32
		} else {
			out[i] = s[i]
		}
	}
	return string(out)
}

func TestDownloadAndVerify_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a slow server so the cancellation has time to fire.
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	_, err := downloadAndVerify(ctx, srv.URL, "doesntmatter")
	require.Error(t, err)
}

func TestExtractTarGz_Happy(t *testing.T) {
	payload := []byte("fake waybill ELF")
	archive := makeTarGz(t, map[string]string{
		"waybill-v0.1.0/LICENSE":   "MIT",
		"waybill-v0.1.0/README.md": "hello",
		"waybill-v0.1.0/waybill":   string(payload),
	})

	src := filepath.Join(t.TempDir(), "src.tar.gz")
	require.NoError(t, os.WriteFile(src, archive, 0o644))
	dest := filepath.Join(t.TempDir(), "waybill")

	require.NoError(t, extractTarGz(src, dest))
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestExtractTarGz_MissingBinary(t *testing.T) {
	archive := makeTarGz(t, map[string]string{
		"waybill-v0.1.0/LICENSE":   "MIT",
		"waybill-v0.1.0/README.md": "hello",
	})

	src := filepath.Join(t.TempDir(), "src.tar.gz")
	require.NoError(t, os.WriteFile(src, archive, 0o644))
	dest := filepath.Join(t.TempDir(), "waybill")

	err := extractTarGz(src, dest)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "waybill binary not found")
}

func TestExtractTarGz_NotAGzip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.tar.gz")
	require.NoError(t, os.WriteFile(src, []byte("definitely not gzip"), 0o644))
	dest := filepath.Join(t.TempDir(), "waybill")

	err := extractTarGz(src, dest)
	require.Error(t, err)
}

// makeZip builds an in-memory .zip from a name → content map, mirroring the
// layout of the upstream Windows release asset.
func makeZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestExtractZip_Happy(t *testing.T) {
	payload := []byte("fake waybill PE")
	archive := makeZip(t, map[string]string{
		"waybill-v0.1.0-alpha.70-x86_64-pc-windows-msvc/LICENSE":     "MIT",
		"waybill-v0.1.0-alpha.70-x86_64-pc-windows-msvc/README.md":   "hello",
		"waybill-v0.1.0-alpha.70-x86_64-pc-windows-msvc/waybill.exe": string(payload),
	})

	src := filepath.Join(t.TempDir(), "src.zip")
	require.NoError(t, os.WriteFile(src, archive, 0o644))
	dest := filepath.Join(t.TempDir(), "waybill.exe")

	require.NoError(t, extractZip(src, dest))
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

func TestExtractZip_MissingBinary(t *testing.T) {
	archive := makeZip(t, map[string]string{
		"waybill-v0.1.0-alpha.70-x86_64-pc-windows-msvc/LICENSE": "MIT",
	})

	src := filepath.Join(t.TempDir(), "src.zip")
	require.NoError(t, os.WriteFile(src, archive, 0o644))

	err := extractZip(src, filepath.Join(t.TempDir(), "waybill.exe"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "waybill binary not found")
}

func TestExtractZip_NotAZip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.zip")
	require.NoError(t, os.WriteFile(src, []byte("definitely not a zip"), 0o644))

	require.Error(t, extractZip(src, filepath.Join(t.TempDir(), "waybill.exe")))
}

// TestExtractBinary_SelectsFormatByAssetName pins the dispatch: upstream ships
// .tar.gz for Unix targets and .zip for Windows, so picking the wrong extractor
// would fail on exactly one platform.
func TestExtractBinary_SelectsFormatByAssetName(t *testing.T) {
	tarPayload := "unix binary"
	tarSrc := filepath.Join(t.TempDir(), "a.tar.gz")
	require.NoError(t, os.WriteFile(tarSrc, makeTarGz(t, map[string]string{
		"waybill-v0.1.0-alpha.70/waybill": tarPayload,
	}), 0o644))

	zipPayload := "windows binary"
	zipSrc := filepath.Join(t.TempDir(), "a.zip")
	require.NoError(t, os.WriteFile(zipSrc, makeZip(t, map[string]string{
		"waybill-v0.1.0-alpha.70/waybill.exe": zipPayload,
	}), 0o644))

	tests := []struct {
		name      string
		src       string
		assetName string
		want      string
	}{
		{
			name:      "unix asset extracts as tar.gz",
			src:       tarSrc,
			assetName: "waybill-v0.1.0-alpha.70-x86_64-unknown-linux-gnu.tar.gz",
			want:      tarPayload,
		},
		{
			name:      "windows asset extracts as zip",
			src:       zipSrc,
			assetName: "waybill-v0.1.0-alpha.70-x86_64-pc-windows-msvc.zip",
			want:      zipPayload,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "waybill")
			require.NoError(t, extractBinary(tt.src, tt.assetName, dest))
			got, err := os.ReadFile(dest)
			require.NoError(t, err)
			assert.Equal(t, tt.want, string(got))
		})
	}
}

// TestAssets_MatchVersion catches versions.go drifting out of internal
// agreement, which is not hypothetical: every asset filename embeds the release
// tag, so a bump that updates Version without regenerating every entry builds a
// download URL from the new tag and the old filename.
//
// That went unnoticed once because a pull request merges cleanly while still
// being wrong. main bumped Version and the three targets it knew about; this
// branch separately added windows/amd64. Neither side touched the other's
// lines, so git produced a merge with a new Version and one stale filename --
// caught only by a 404 on the Windows runner, on the one platform no
// contributor runs locally.
//
// Regenerate with scripts/bump-waybill.sh rather than editing by hand.
func TestAssets_MatchVersion(t *testing.T) {
	require.NotEmpty(t, Version)
	for target, a := range assets {
		assert.Contains(t, a.Filename, "v"+Version,
			"asset for %s is %q, which does not carry Version %q -- versions.go is internally "+
				"inconsistent; re-run scripts/bump-waybill.sh", target, a.Filename, Version)
	}
}

// TestAssets_CoverSupportedPlatforms guards against a bump that silently drops
// a target from versions.go.
func TestAssets_CoverSupportedPlatforms(t *testing.T) {
	for _, target := range []string{"darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64"} {
		a, ok := assets[target]
		require.True(t, ok, "no waybill asset registered for %s", target)
		assert.NotEmpty(t, a.Filename, "%s asset has no filename", target)
		assert.Len(t, a.SHA256, 64, "%s asset sha256 is not a full digest", target)

		wantExt := ".tar.gz"
		if strings.HasPrefix(target, "windows/") {
			wantExt = ".zip"
		}
		assert.True(t, strings.HasSuffix(a.Filename, wantExt),
			"%s asset %q should end in %s", target, a.Filename, wantExt)
	}
}

func TestEnsureAvailable_EnvOverrideHonored(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "custom-waybill")
	require.NoError(t, os.WriteFile(tmp, []byte("fake"), 0o755))
	t.Setenv(EnvBinOverride, tmp)
	t.Setenv(EnvBinOverrideLegacy, "")

	got, err := EnsureAvailable(context.Background())
	require.NoError(t, err)
	assert.Equal(t, tmp, got)
}

func TestEnsureAvailable_EnvOverrideMissingFile(t *testing.T) {
	t.Setenv(EnvBinOverride, filepath.Join(t.TempDir(), "does-not-exist"))
	t.Setenv(EnvBinOverrideLegacy, "")

	_, err := EnsureAvailable(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvBinOverride)
}

// TestEnsureAvailable_LegacyEnvOverrideHonored covers back-compat: the
// pre-rename KUSARI_MIKEBOM_BIN is still honored when set on its own.
func TestEnsureAvailable_LegacyEnvOverrideHonored(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "custom-waybill")
	require.NoError(t, os.WriteFile(tmp, []byte("fake"), 0o755))
	t.Setenv(EnvBinOverride, "")
	t.Setenv(EnvBinOverrideLegacy, tmp)

	got, err := EnsureAvailable(context.Background())
	require.NoError(t, err)
	assert.Equal(t, tmp, got)
}

// TestEnsureAvailable_NewEnvOverrideWinsOverLegacy documents the precedence
// when both names are set: the current name takes effect.
func TestEnsureAvailable_NewEnvOverrideWinsOverLegacy(t *testing.T) {
	current := filepath.Join(t.TempDir(), "current-waybill")
	require.NoError(t, os.WriteFile(current, []byte("current"), 0o755))
	legacy := filepath.Join(t.TempDir(), "legacy-waybill")
	require.NoError(t, os.WriteFile(legacy, []byte("legacy"), 0o755))
	t.Setenv(EnvBinOverride, current)
	t.Setenv(EnvBinOverrideLegacy, legacy)

	got, err := EnsureAvailable(context.Background())
	require.NoError(t, err)
	assert.Equal(t, current, got)
}

// TestEnsureAvailable_LegacyEnvOverrideMissingFile names the legacy env var
// in the error when that is the one supplying the (bad) path.
func TestEnsureAvailable_LegacyEnvOverrideMissingFile(t *testing.T) {
	t.Setenv(EnvBinOverride, "")
	t.Setenv(EnvBinOverrideLegacy, filepath.Join(t.TempDir(), "does-not-exist"))

	_, err := EnsureAvailable(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvBinOverrideLegacy)
}

// useTempHome points os.UserHomeDir at a fresh directory for the duration of
// the test, so the cache lookup never sees the real ~/.kusari.
//
// Setting only HOME is not enough: os.UserHomeDir reads USERPROFILE on Windows.
// With a HOME-only override these tests fall through to the developer's (or CI
// runner's) actual home, which for the cache-hit test meant a real 23 MB
// download from GitHub on every run.
func useTempHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

func TestEnsureAvailable_NoAutoInstallFailsWithoutCache(t *testing.T) {
	// Force a fresh home so the cache lookup misses.
	useTempHome(t)
	t.Setenv(EnvBinOverride, "")
	t.Setenv(EnvBinOverrideLegacy, "")
	t.Setenv(EnvNoAutoInstall, "1")

	_, err := EnsureAvailable(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvNoAutoInstall)
}

func TestEnsureAvailable_CacheHitSkipsDownload(t *testing.T) {
	useTempHome(t)
	t.Setenv(EnvBinOverride, "")
	t.Setenv(EnvBinOverrideLegacy, "")
	// EnsureAvailable consults the cache before this gate, so a hit still
	// succeeds while a miss fails fast instead of downloading. Without it a
	// broken cache lookup turns this test into a silent network fetch.
	t.Setenv(EnvNoAutoInstall, "1")

	// Pre-populate the cache at the path the production code computes, so the
	// test cannot drift from the platform-specific naming (waybill-<v>.exe on
	// Windows) the way a hand-built path did.
	cachePath, err := cachedBinaryPath()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o755))
	require.NoError(t, os.WriteFile(cachePath, []byte("fake"), 0o755))

	got, err := EnsureAvailable(context.Background())
	require.NoError(t, err)
	assert.Equal(t, cachePath, got)
}

// TestCachedBinaryPath_PlatformSuffix pins the naming that the cache-hit test
// now derives rather than asserts. Windows refuses to execute a file without an
// executable extension, so the suffix is required there and must not appear
// elsewhere.
func TestCachedBinaryPath_PlatformSuffix(t *testing.T) {
	home := useTempHome(t)

	got, err := cachedBinaryPath()
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".kusari", "bin"), filepath.Dir(got))
	if runtime.GOOS == "windows" {
		assert.Equal(t, "waybill-"+Version+".exe", filepath.Base(got))
	} else {
		assert.Equal(t, "waybill-"+Version, filepath.Base(got))
	}
}

func TestIsUsableBinary(t *testing.T) {
	dir := t.TempDir()

	populated := filepath.Join(dir, "waybill")
	require.NoError(t, os.WriteFile(populated, []byte("ELF..."), 0o755))
	assert.True(t, isUsableBinary(populated))

	// A zero-length file is someone else's failed install, not one to adopt.
	empty := filepath.Join(dir, "empty")
	require.NoError(t, os.WriteFile(empty, nil, 0o755))
	assert.False(t, isUsableBinary(empty), "a zero-length file must not count as installed")

	assert.False(t, isUsableBinary(filepath.Join(dir, "absent")))
	assert.False(t, isUsableBinary(dir), "a directory must not count as installed")
}
