// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kusaridev/kusari-cli/v2/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bundle is the fully-built upload artifact, unpacked for inspection.
type bundle struct {
	files map[string]string
	meta  api.BundleMeta
	patch string
	size  int64
}

// buildBundle runs the same local pipeline scan() does — temp dirs, metadata,
// diff, tarball — stopping short of anything that needs credentials or the
// network. This is everything the CLI does on the user's machine, so it is
// where any platform-specific breakage shows up.
func buildBundle(t *testing.T, repoDir, rev string, full bool) bundle {
	t.Helper()

	tempDir := t.TempDir()
	tarballDir = tempDir
	workingDir = filepath.Join(tempDir, workingDirName)
	require.NoError(t, os.Mkdir(workingDir, 0700))
	metaName = filepath.Join(workingDir, metaFile)
	patchName = filepath.Join(workingDir, patchFile)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})
	require.NoError(t, os.Chdir(repoDir))

	_, err = createMeta(rev, full, "")
	require.NoError(t, err, "createMeta")

	if !full {
		require.NoError(t, generateDiff(rev), "generateDiff")
	}

	size, err := packageDirectory(full)
	require.NoError(t, err, "packageDirectory")

	b := bundle{files: extractTarballFiles(t, filepath.Join(tarballDir, tarballName)), size: size}

	rawMeta, ok := b.files[metaFile]
	require.True(t, ok, "bundle is missing %s; has %v", metaFile, keysOf(b.files))
	require.NoError(t, json.Unmarshal([]byte(rawMeta), &b.meta), "bundle metadata is not valid JSON")

	if !full {
		b.patch, ok = b.files[patchFile]
		require.True(t, ok, "bundle is missing %s; has %v", patchFile, keysOf(b.files))
	}
	return b
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// initRepo creates a git repo with the given extra config applied before any
// content is written, so settings like core.autocrlf take effect on checkout.
func initRepo(t *testing.T, dir string, config ...[2]string) {
	t.Helper()
	runCmd(t, dir, "git", "init")
	runCmd(t, dir, "git", "config", "user.email", "test@example.com")
	runCmd(t, dir, "git", "config", "user.name", "Test User")
	// Keep the CI runner's global config from changing what these tests see.
	runCmd(t, dir, "git", "config", "core.quotePath", "true")
	for _, kv := range config {
		runCmd(t, dir, "git", "config", kv[0], kv[1])
	}
}

// TestBundle_EndToEnd walks the complete local pipeline for a diff scan and
// asserts the upload artifact is well-formed: every working-tree file present
// under a slash-separated name, metadata that parses, and a patch describing
// the uncommitted work.
func TestBundle_EndToEnd(t *testing.T) {
	repoDir := t.TempDir()
	initRepo(t, repoDir)

	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "pkg", "nested dir"), 0755))
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(repoDir, "pkg", "nested dir", "lib.go"), "package lib\n")
	writeFile(t, filepath.Join(repoDir, ".gitignore"), "*.log\n")
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial")

	// Uncommitted work: one modified tracked file, one new untracked file, one
	// ignored file that must not travel.
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n\nfunc main() { println(\"hi\") }\n")
	writeFile(t, filepath.Join(repoDir, "added.go"), "package main\n")
	writeFile(t, filepath.Join(repoDir, "debug.log"), "noise\n")

	b := buildBundle(t, repoDir, "HEAD", false)

	assert.Positive(t, b.size)

	for _, want := range []string{"main.go", "added.go", ".gitignore", "pkg/nested dir/lib.go"} {
		assert.Contains(t, b.files, want, "missing from bundle")
	}
	assert.NotContains(t, b.files, "debug.log", "gitignored file must not be bundled")

	// Tar names are slash-separated on every platform, including Windows.
	for name := range b.files {
		assert.NotContains(t, name, `\`, "tar entry name must not contain a backslash")
	}

	// The bundle carries the working tree, not HEAD.
	assert.Contains(t, b.files["main.go"], `println("hi")`)

	assert.Equal(t, "diff", b.meta.ScanType)
	assert.Equal(t, filepath.Base(repoDir), b.meta.DirName)
	assert.NotEmpty(t, b.meta.CurrentBranch)
	assert.NotEqual(t, "HEAD", b.meta.CurrentBranch)
	assert.NotEmpty(t, b.meta.CommitSHA)
	assert.True(t, b.meta.GitDirty)
	assert.Contains(t, b.meta.ChangedFiles, "main.go")
	assert.Contains(t, b.meta.ChangedFiles, "added.go")
	assert.Contains(t, b.meta.ChangedFileHashes, "main.go")

	assert.Contains(t, b.patch, "diff --git a/main.go b/main.go")
	assert.Contains(t, b.patch, "+++ b/added.go", "untracked files must appear in the patch")
}

// TestBundle_EndToEnd_FullScan covers the risk-check path, which ships no patch.
func TestBundle_EndToEnd_FullScan(t *testing.T) {
	repoDir := t.TempDir()
	initRepo(t, repoDir)
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n")
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial")

	b := buildBundle(t, repoDir, "", true)

	assert.Contains(t, b.files, "main.go")
	assert.Contains(t, b.files, metaFile)
	assert.NotContains(t, b.files, patchFile, "full scans carry no patch")
	assert.Equal(t, "full", b.meta.ScanType)
}

// TestBundle_LineEndingsUnderAutoCRLF documents how the bundle looks when git
// is configured the way the Git for Windows installer configures it by default
// (core.autocrlf=true).
//
// git renormalizes to LF when producing a diff but leaves CRLF in the working
// tree, so the tarball and the patch disagree about line endings. That is
// inherent to "archive the working tree, diff through git" and is not specific
// to any one platform — this test pins the behavior so a change to it is
// visible rather than silent.
func TestBundle_LineEndingsUnderAutoCRLF(t *testing.T) {
	repoDir := t.TempDir()
	initRepo(t, repoDir, [2]string{"core.autocrlf", "true"})

	writeFile(t, filepath.Join(repoDir, "a.txt"), "line one\nline two\nline three\n")
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial")

	// Re-checkout so autocrlf applies its CRLF conversion to the working tree,
	// reproducing what a Windows user sees after a fresh clone.
	require.NoError(t, os.Remove(filepath.Join(repoDir, "a.txt")))
	runCmd(t, repoDir, "git", "checkout", "--", "a.txt")

	onDisk, err := os.ReadFile(filepath.Join(repoDir, "a.txt"))
	require.NoError(t, err)
	if !strings.Contains(string(onDisk), "\r\n") {
		t.Skip("git did not apply autocrlf conversion in this environment")
	}

	writeFile(t, filepath.Join(repoDir, "a.txt"), "line one\r\nline two CHANGED\r\nline three\r\n")

	b := buildBundle(t, repoDir, "HEAD", false)

	// The archived source is the working tree byte-for-byte, CRLF included.
	assert.Contains(t, b.files["a.txt"], "\r\n", "tarball should carry working-tree bytes verbatim")
	// The patch is what git emits, which is renormalized to LF.
	assert.NotContains(t, b.patch, "\r\n", "git renormalizes the diff to LF under autocrlf")

	t.Logf("bundle line endings: tarball=CRLF patch=LF (autocrlf=true); "+
		"patch is %d bytes", len(b.patch))
}

// TestBundle_SubmoduleFilesUseSlashPaths covers the one path where an on-disk
// filename becomes a tar entry name. git reports a submodule as a single
// directory entry, so the archiver walks it, and on Windows that walk yields
// backslash-separated relative paths. Consumers reject entry names containing a
// backslash outright, so the conversion to slashes is load-bearing rather than
// cosmetic.
func TestBundle_SubmoduleFilesUseSlashPaths(t *testing.T) {
	// The inner repo the submodule points at.
	innerDir := t.TempDir()
	initRepo(t, innerDir)
	require.NoError(t, os.MkdirAll(filepath.Join(innerDir, "deep", "inner dir"), 0755))
	writeFile(t, filepath.Join(innerDir, "deep", "inner dir", "vendored.go"), "package vendored\n")
	runCmd(t, innerDir, "git", "add", ".")
	runCmd(t, innerDir, "git", "commit", "-m", "inner")

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n")
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial")

	add := exec.Command("git", "-c", "protocol.file.allow=always",
		"submodule", "add", innerDir, "vendor mod")
	add.Dir = repoDir
	if out, err := add.CombinedOutput(); err != nil {
		t.Skipf("git submodule add unavailable in this environment: %v\n%s", err, out)
	}
	runCmd(t, repoDir, "git", "commit", "-m", "add submodule")
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n\nfunc main() {}\n")

	b := buildBundle(t, repoDir, "HEAD", false)

	var sawSubmoduleFile bool
	for name := range b.files {
		assert.NotContains(t, name, `\`, "tar entry name must not contain a backslash")
		if strings.HasPrefix(name, "vendor mod/") {
			sawSubmoduleFile = true
		}
	}
	assert.True(t, sawSubmoduleFile,
		"expected submodule contents in the bundle, got %v", keysOf(b.files))
	assert.Contains(t, b.files, "vendor mod/deep/inner dir/vendored.go")
	// The submodule's own git metadata must not travel. Registered submodules
	// have a ".git" *file* holding a "gitdir:" path from the scanning machine,
	// so matching only on a "/.git/" directory prefix would miss it.
	for name := range b.files {
		assert.NotContains(t, name, "/.git/", "submodule .git directory must be skipped")
		assert.False(t, strings.HasSuffix(name, "/.git"),
			"submodule .git pointer file must be skipped, got %q", name)
	}
}

// TestBundle_TarballIsWellFormed re-reads the archive with a strict reader to
// confirm every header is internally consistent (sizes match payloads, no
// truncated members) rather than only that the names look right.
func TestBundle_TarballIsWellFormed(t *testing.T) {
	repoDir := t.TempDir()
	initRepo(t, repoDir)
	for _, name := range []string{"a.go", "b.go", "c.go"} {
		writeFile(t, filepath.Join(repoDir, name), strings.Repeat("package main\n", 500))
	}
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial")
	// A diff scan needs something to diff.
	writeFile(t, filepath.Join(repoDir, "a.go"), strings.Repeat("package main\n", 400))

	buildBundle(t, repoDir, "HEAD", false)

	f, err := os.Open(filepath.Join(tarballDir, tarballName))
	require.NoError(t, err)
	defer func() {
		_ = f.Close()
	}()

	tr := tar.NewReader(bzip2.NewReader(f))
	var members int
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "tar header %d is malformed", members)

		n, err := io.Copy(io.Discard, tr)
		require.NoError(t, err, "payload of %s is truncated", hdr.Name)
		require.Equal(t, hdr.Size, n, "declared size of %s does not match its payload", hdr.Name)
		members++
	}
	assert.GreaterOrEqual(t, members, 4, "expected sources plus metadata")
}

// TestGitAvailable fails loudly if the environment has no usable git, which
// would otherwise surface as a confusing cascade of failures above.
func TestGitAvailable(t *testing.T) {
	out, err := exec.Command("git", "--version").Output()
	require.NoError(t, err, "git must be on PATH for the repo package tests")
	t.Logf("using %s", strings.TrimSpace(string(out)))
}

// TestBundle_MetadataNotShadowedByRepoFile covers a repository that happens to
// contain a file named like the Inspector metadata.
//
// This has to be the bundle's own metadata, not the repo's file. The failure it
// guards against is silent rather than loud: the repo's file is very likely
// valid JSON, so the backend unmarshals it into a zero-valued BundleMeta and
// analyses the upload against meaningless values instead of rejecting it.
func TestBundle_MetadataNotShadowedByRepoFile(t *testing.T) {
	repoDir := t.TempDir()
	initRepo(t, repoDir)
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n")
	writeFile(t, filepath.Join(repoDir, metaFile), `{"scan_type":"impostor","dir_name":"impostor"}`)
	writeFile(t, filepath.Join(repoDir, patchFile), "not a real patch\n")
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial")
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n\nfunc main() {}\n")

	b := buildBundle(t, repoDir, "HEAD", false)

	assert.Equal(t, "diff", b.meta.ScanType, "bundle metadata was shadowed by the repository's own file")
	assert.Equal(t, filepath.Base(repoDir), b.meta.DirName)
	assert.NotContains(t, b.files[metaFile], "impostor")
	assert.Contains(t, b.patch, "diff --git", "patch was shadowed by the repository's own file")

	// Exactly one entry per reserved name, so every consumer agrees on which
	// one it got regardless of whether it keeps the first or the last.
	assert.Equal(t, 1, countTarEntries(t, filepath.Join(tarballDir, tarballName), metaFile))
	assert.Equal(t, 1, countTarEntries(t, filepath.Join(tarballDir, tarballName), patchFile))
}

// TestBundle_SymlinkedDirectoryContentsIncluded pins that a directory reached
// through a symlink is archived. The previous walk used filepath.WalkDir, which
// stats with Lstat and silently treats a symlinked directory as a non-regular
// file, dropping everything beneath it from the scan without a word.
func TestBundle_SymlinkedDirectoryContentsIncluded(t *testing.T) {
	requireSymlinks(t)

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "real", "nested"), 0755))
	writeFile(t, filepath.Join(repoDir, "real", "top.go"), "package real\n")
	writeFile(t, filepath.Join(repoDir, "real", "nested", "deep.go"), "package nested\n")
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n")
	require.NoError(t, os.Symlink(filepath.Join(repoDir, "real"), filepath.Join(repoDir, "linkdir")))
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial")
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n\nfunc main() {}\n")

	b := buildBundle(t, repoDir, "HEAD", false)

	assert.Contains(t, b.files, "linkdir/top.go", "symlinked directory contents dropped; got %v", keysOf(b.files))
	assert.Contains(t, b.files, "linkdir/nested/deep.go", "nested contents under a symlinked directory dropped")
	assert.Equal(t, "package real\n", b.files["linkdir/top.go"])
}

// TestBundle_SymlinkLoopTerminates checks that following symlinked directories
// cannot spin forever. Dereferencing links is only safe with cycle detection,
// and a self-referential link is the cheapest way to prove it is there.
func TestBundle_SymlinkLoopTerminates(t *testing.T) {
	requireSymlinks(t)

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "a", "b"), 0755))
	writeFile(t, filepath.Join(repoDir, "a", "file.go"), "package a\n")
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n")
	// a/b/loop points back at a, so a naive walk recurses forever.
	require.NoError(t, os.Symlink(filepath.Join(repoDir, "a"), filepath.Join(repoDir, "a", "b", "loop")))
	require.NoError(t, os.Symlink(filepath.Join(repoDir, "a"), filepath.Join(repoDir, "linkdir")))
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial")
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n\nfunc main() {}\n")

	done := make(chan struct{})
	go func() {
		defer close(done)
		b := buildBundle(t, repoDir, "HEAD", false)
		assert.Contains(t, b.files, "linkdir/file.go")
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("packaging did not terminate on a symlink loop")
	}
}

// countTarEntries counts how many members of the archive carry a given name.
func countTarEntries(t *testing.T, tarballPath, name string) int {
	t.Helper()
	f, err := os.Open(tarballPath)
	require.NoError(t, err)
	defer func() {
		_ = f.Close()
	}()

	tr := tar.NewReader(bzip2.NewReader(f))
	n := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		if hdr.Name == name {
			n++
		}
	}
	return n
}

// TestBundle_SymlinkEscapingRepoExcluded is the counterweight to following
// symlinked directories at all.
//
// git will happily track a symlink pointing anywhere on the machine. Since the
// bundle is uploaded, descending through such a link would sweep an unrelated
// tree -- /etc, a home directory, a mounted secret -- into someone else's
// storage. Links that stay inside the repository are followed; links that leave
// it are reported and skipped.
func TestBundle_SymlinkEscapingRepoExcluded(t *testing.T) {
	requireSymlinks(t)

	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "SECRET.txt"),
		[]byte("private key material\n"), 0600))

	repoDir := t.TempDir()
	initRepo(t, repoDir)
	require.NoError(t, os.MkdirAll(filepath.Join(repoDir, "real"), 0755))
	writeFile(t, filepath.Join(repoDir, "real", "x.go"), "package real\n")
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n")
	require.NoError(t, os.Symlink(outside, filepath.Join(repoDir, "extdir")))
	require.NoError(t, os.Symlink(filepath.Join(repoDir, "real"), filepath.Join(repoDir, "linkdir")))
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial")
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n\nfunc main() {}\n")

	b := buildBundle(t, repoDir, "HEAD", false)

	for name, content := range b.files {
		assert.NotContains(t, name, "SECRET", "content from outside the repository was bundled")
		assert.NotContains(t, content, "private key material",
			"content from outside the repository was bundled as %q", name)
	}
	// The in-repo symlink is still followed; this is a boundary, not a ban.
	assert.Contains(t, b.files, "linkdir/x.go")
}

// requireSymlinks skips the calling test unless this machine can actually
// create symlinks.
//
// Windows can, but only with Developer Mode enabled or an elevated shell, so
// the capability is probed rather than assumed absent from the whole platform.
// Skipping every Windows run outright would leave the symlink handling -- the
// part of the archiver most likely to behave differently there -- with no
// coverage on the one OS where it is hardest to reason about.
func requireSymlinks(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("probe"), 0600); err != nil {
		t.Fatalf("failed to write symlink probe target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		t.Skipf("this machine cannot create symlinks (%v); on Windows enable Developer Mode "+
			"(Settings > System > For developers) or run from an elevated shell to exercise this test", err)
	}
}

// TestBundle_UntrackedBinaryExcludedFromPatch pins the reason a stray build
// artifact used to sink a whole scan.
//
// git diff --binary inlines a binary file into the patch as base85, so an
// untracked 30 MB executable produced a ~37 MB patch -- roughly 19 million
// tokens, past any model's limit, and the analyzer rejected the upload with an
// error naming nothing that led back to the cause.
//
// It also records the half this does NOT fix: the tarball is built from its own
// git ls-files call in packageDirectory, so the binary still travels. Only the
// patch is spared.
func TestBundle_UntrackedBinaryExcludedFromPatch(t *testing.T) {
	repoDir := t.TempDir()
	initRepo(t, repoDir)
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n")
	runCmd(t, repoDir, "git", "add", ".")
	runCmd(t, repoDir, "git", "commit", "-m", "initial")

	// A real text change, so the diff is not empty on its own merits.
	writeFile(t, filepath.Join(repoDir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(repoDir, "added.go"), "package main\n")

	// An untracked build artifact: NUL bytes make it binary by git's heuristic.
	binary := make([]byte, 512*1024)
	for i := range binary {
		binary[i] = byte(i % 256) // includes 0x00
	}
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "app.exe"), binary, 0644))

	b := buildBundle(t, repoDir, "HEAD", false)

	assert.NotContains(t, b.patch, "app.exe", "an untracked binary must not reach the patch")
	assert.NotContains(t, b.patch, "GIT binary patch", "no base85 blob belongs in the patch")
	assert.Less(t, len(b.patch), 8*1024,
		"patch should stay small; a binary blob would dwarf it (got %d bytes)", len(b.patch))

	// Text changes are untouched by the filter.
	assert.Contains(t, b.patch, "diff --git a/main.go b/main.go")
	assert.Contains(t, b.patch, "+++ b/added.go")

	// The tarball is assembled separately and still carries the binary. If this
	// ever changes, it is a deliberate decision about upload size, not a
	// side-effect of the patch filter.
	assert.Contains(t, b.files, "app.exe",
		"documenting current behaviour: the tarball is built from its own file list")
}

func TestIsBinaryFile(t *testing.T) {
	dir := t.TempDir()

	write := func(name string, content []byte) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, content, 0644))
		return p
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"go source", write("a.go", []byte("package main\n\nfunc main() {}\n")), false},
		{"empty file", write("empty", nil), false},
		{"utf-8 text", write("u.txt", []byte("héllo wörld — ok\n")), false},
		{"crlf text", write("crlf.txt", []byte("line one\r\nline two\r\n")), false},
		{"nul in the first bytes", write("bin1", []byte{'M', 'Z', 0x00, 0x01}), true},
		{
			// git only inspects the first 8000 bytes, so a NUL past that window
			// is not treated as binary. Matching that keeps us consistent with
			// what git itself will put in the diff.
			name: "nul beyond the sniff window",
			path: write("bin2", append(bytes.Repeat([]byte("a"), binarySniffLen+10), 0x00)),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isBinaryFile(tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	_, err := isBinaryFile(filepath.Join(dir, "does-not-exist"))
	assert.Error(t, err, "a missing file must report an error, not a verdict")
}
