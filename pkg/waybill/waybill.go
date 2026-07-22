// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

// Package waybill lazy-installs and invokes a pinned version of Waybill
// (https://github.com/kusari-oss/waybill) as a prerequisite of kusari
// subcommands that need it.
package waybill

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
)

type asset struct {
	Filename string
	SHA256   string
}

// EnvBinOverride lets a user point at a pre-installed waybill (air-gapped,
// local dev builds, test fixtures). When set, the cache + download path is
// skipped entirely.
const EnvBinOverride = "KUSARI_WAYBILL_BIN"

// EnvBinOverrideLegacy is the pre-rename name for EnvBinOverride, retained
// permanently for back-compat. When both are set, EnvBinOverride wins and
// this one is ignored; when only this one is set it is honored, with a
// deprecation warning.
const EnvBinOverrideLegacy = "KUSARI_MIKEBOM_BIN"

// EnvNoAutoInstall, when "1", causes EnsureAvailable to error instead of
// downloading. Useful in CI / regulated environments.
const EnvNoAutoInstall = "KUSARI_NO_AUTO_INSTALL"

// downloadTimeout bounds the total wall-clock time for fetching a Waybill
// release asset (connect + TLS + headers + body). Generous enough for slow
// links to complete a multi-MB download; short enough that a hung server
// surfaces an error in reasonable time.
const downloadTimeout = 2 * time.Minute

// legacyBinWarn ensures the KUSARI_MIKEBOM_BIN deprecation notice is printed
// at most once per process.
var legacyBinWarn sync.Once

// binOverride resolves a user-supplied binary path from the environment. The
// current name (KUSARI_WAYBILL_BIN) takes precedence over the deprecated
// KUSARI_MIKEBOM_BIN when both are set. The second return value is the env
// var name that actually supplied the value, for use in error messages;
// it is empty when neither is set. A one-time deprecation warning is emitted
// when the legacy name is the one in effect.
func binOverride() (path, envName string) {
	if p := os.Getenv(EnvBinOverride); p != "" {
		return p, EnvBinOverride
	}
	if p := os.Getenv(EnvBinOverrideLegacy); p != "" {
		legacyBinWarn.Do(func() {
			fmt.Fprintf(os.Stderr,
				"kusari: %s is deprecated and still honored; prefer %s (which wins if both are set)\n",
				EnvBinOverrideLegacy, EnvBinOverride)
		})
		return p, EnvBinOverrideLegacy
	}
	return "", ""
}

// EnsureAvailable returns the filesystem path to a verified waybill binary
// matching the pinned Version, installing it on first use.
func EnsureAvailable(ctx context.Context) (string, error) {
	if p, envName := binOverride(); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("%s=%q: %w", envName, p, err)
		}
		return p, nil
	}

	binPath, err := cachedBinaryPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	if os.Getenv(EnvNoAutoInstall) == "1" {
		return "", fmt.Errorf(
			"waybill %s not installed at %s and auto-install is disabled (%s=1). "+
				"Download manually from %s or set %s=/path/to/waybill",
			Version, binPath, EnvNoAutoInstall, releasePageURL(), EnvBinOverride,
		)
	}

	return install(ctx, binPath)
}

// Run invokes waybill with the given args, wiring stdio through. The caller
// owns flag parsing; everything in args is passed verbatim.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	binPath, err := EnsureAvailable(ctx)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, binPath, args...)
	cmd.Args = append([]string{"waybill"}, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

func cachedBinaryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kusari", "bin", "waybill-"+Version), nil
}

func currentAsset() (asset, error) {
	key := runtime.GOOS + "/" + runtime.GOARCH
	a, ok := assets[key]
	if !ok {
		return asset{}, fmt.Errorf("waybill %s has no published asset for %s; "+
			"see %s or set %s=/path/to/waybill", Version, key, releasePageURL(), EnvBinOverride)
	}
	return a, nil
}

func releasePageURL() string {
	return fmt.Sprintf("https://github.com/%s/releases/tag/v%s", Repo, Version)
}

func assetURL(filename string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", Repo, Version, filename)
}

func install(ctx context.Context, binPath string) (string, error) {
	a, err := currentAsset()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(binPath), 0o755); err != nil {
		return "", err
	}

	fmt.Fprintf(os.Stderr, "kusari: Waybill %s not found locally, downloading from %s\n", Version, Repo)
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Writer = os.Stderr
	s.Suffix = " downloading " + a.Filename
	s.Start()
	defer s.Stop()

	archive, err := downloadAndVerify(ctx, assetURL(a.Filename), a.SHA256)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(archive) }()

	s.Suffix = " extracting"
	tmp := binPath + ".tmp"
	if err := extractTarGz(archive, tmp); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return "", fmt.Errorf("chmod %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, binPath); err != nil {
		return "", fmt.Errorf("rename %s -> %s: %w", tmp, binPath, err)
	}

	s.Stop()
	fmt.Fprintf(os.Stderr, "kusari: installed Waybill %s to %s\n", Version, binPath)
	return binPath, nil
}

func downloadAndVerify(ctx context.Context, url, wantHex string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	f, err := os.CreateTemp("", "waybill-*.tar.gz")
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}

	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, wantHex) {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("checksum mismatch for %s: got %s, want %s", url, got, wantHex)
	}
	return f.Name(), nil
}

func extractTarGz(archivePath, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return fmt.Errorf("waybill binary not found in archive")
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != "waybill" {
			continue
		}
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return err
		}
		return out.Close()
	}
}
