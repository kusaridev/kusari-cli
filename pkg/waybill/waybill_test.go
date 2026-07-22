// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package waybill

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestEnsureAvailable_NoAutoInstallFailsWithoutCache(t *testing.T) {
	// Force a fresh HOME so the cache lookup misses.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvBinOverride, "")
	t.Setenv(EnvBinOverrideLegacy, "")
	t.Setenv(EnvNoAutoInstall, "1")

	_, err := EnsureAvailable(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), EnvNoAutoInstall)
}

func TestEnsureAvailable_CacheHitSkipsDownload(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvBinOverride, "")
	t.Setenv(EnvBinOverrideLegacy, "")

	// Pre-populate the expected cache path so EnsureAvailable returns it
	// without attempting any network I/O.
	cacheDir := filepath.Join(home, ".kusari", "bin")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	cachePath := filepath.Join(cacheDir, "waybill-"+Version)
	require.NoError(t, os.WriteFile(cachePath, []byte("fake"), 0o755))

	got, err := EnsureAvailable(context.Background())
	require.NoError(t, err)
	assert.Equal(t, cachePath, got)
}
