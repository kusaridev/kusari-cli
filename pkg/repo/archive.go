// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package repo

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dsnet/compress/bzip2"
)

// zeroReader is an infinite source of NUL bytes, used to pad a tar member when
// the file shrank between stat and read.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// archiveEntry is a single member of the bundle we upload.
type archiveEntry struct {
	// name is the path recorded inside the archive. Tar names are always
	// slash-separated, on every platform.
	name string
	// path is the on-disk location the contents are read from.
	path string
}

// writeTarBz2 writes entries to outPath as a bzip2-compressed tar archive.
//
// root is the resolved repository root. Directories reached through a symlink
// are only descended into when they resolve inside it -- see addDir.
//
// This is a pure-Go replacement for shelling out to tar(1) and bzip2(1), which
// are not present on a stock Windows install (and whose flags differ between
// GNU tar and the bsdtar that ships with Windows).
func writeTarBz2(outPath, root string, entries []archiveEntry) (err error) {
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("error creating archive: %w", err)
	}
	defer func() {
		closeErr := out.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("error closing archive: %w", closeErr)
		}
	}()

	// bzip2(1) defaults to a 900k block size, so match it to keep bundle sizes
	// comparable to what the shell-out produced.
	bz, err := bzip2.NewWriter(out, &bzip2.WriterConfig{Level: bzip2.BestCompression})
	if err != nil {
		return fmt.Errorf("error initializing bzip2 writer: %w", err)
	}
	tw := tar.NewWriter(bz)

	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if err := addPath(tw, e.name, e.path, root, seen); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return fmt.Errorf("error finalizing tar: %w", err)
	}
	if err := bz.Close(); err != nil {
		return fmt.Errorf("error compressing archive: %w", err)
	}
	return nil
}

// addPath writes a single path into the archive under the given tar name.
// Directories (git reports submodules as a single directory entry) are walked
// recursively. Anything that is not a regular file after symlink resolution is
// skipped, matching the old `tar --dereference` behavior.
func addPath(tw *tar.Writer, name, diskPath, root string, seen map[string]bool) error {
	// Stat, not Lstat: the old invocation passed --dereference, so symlinks are
	// stored as copies of their target rather than as links.
	fi, err := os.Stat(diskPath)
	if err != nil {
		// A file listed by git can vanish between listing and archiving, and a
		// dangling symlink cannot be dereferenced. Neither is worth failing the
		// whole scan over.
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("error reading %s: %w", diskPath, err)
	}

	if fi.IsDir() {
		return addDir(tw, name, diskPath, root, seen, map[string]bool{})
	}
	if !fi.Mode().IsRegular() {
		return nil
	}
	return addFile(tw, name, diskPath, fi, seen)
}

// addDir recursively adds the regular files under diskPath.
//
// Directories reached through a symlink are walked like any other, because the
// old invocation passed tar --dereference and a symlinked directory is a
// perfectly ordinary way to lay out a repository. filepath.WalkDir cannot do
// this -- it stats with Lstat and never descends through a link -- so the walk
// is written out by hand.
//
// chain holds the resolved directories on the current path from the root, which
// is what stops a symlink loop (a -> b -> a) from recursing forever. It is
// scoped to the branch rather than the whole walk, so a directory legitimately
// reachable by two different routes is still archived under both.
//
// Nested git metadata is skipped: it is bloat at best, and the ".git" of a
// registered submodule is a file pointing at a path on the machine that ran the
// scan, which means nothing anywhere else.
func addDir(tw *tar.Writer, name, diskPath, root string, seen, chain map[string]bool) error {
	// Absolute first, then resolve: EvalSymlinks preserves the relativity of its
	// input, and a relative result cannot be compared against the absolute root.
	// Resolving also lets cycle detection compare real identities rather than
	// the routes taken to reach them.
	abs, err := filepath.Abs(diskPath)
	if err != nil {
		return nil //nolint:nilerr // best-effort traversal
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Dangling or unreadable: skip rather than abort the scan.
		return nil //nolint:nilerr // best-effort traversal
	}
	// A repository can contain a symlink to a directory anywhere on the
	// machine. Following one would sweep that entire tree into a bundle bound
	// for upload -- /etc, a home directory, a mounted secret -- so only links
	// that stay inside the repository are descended into. The user is told what
	// was left out rather than having it silently disappear.
	if !withinRoot(root, real) {
		fmt.Fprintf(os.Stderr,
			"Warning: %s resolves to %s, outside the repository; its contents are excluded from this scan\n",
			name, real)
		return nil
	}
	if chain[real] {
		return nil
	}
	chain[real] = true
	defer delete(chain, real)

	dirEntries, err := os.ReadDir(real)
	if err != nil {
		return nil //nolint:nilerr // best-effort traversal
	}

	for _, d := range dirEntries {
		// A submodule's .git is a directory when cloned standalone and a
		// "gitdir:" pointer file when registered as a submodule; skip both.
		if d.Name() == ".git" {
			continue
		}

		childPath := filepath.Join(real, d.Name())
		childName := path.Join(name, d.Name())

		// Stat, not the DirEntry's type: a symlink has to be resolved to know
		// whether it leads to a directory or a file.
		fi, err := os.Stat(childPath)
		if err != nil {
			continue
		}
		switch {
		case fi.IsDir():
			if err := addDir(tw, childName, childPath, root, seen, chain); err != nil {
				return err
			}
		case fi.Mode().IsRegular():
			if err := addFile(tw, childName, childPath, fi, seen); err != nil {
				return err
			}
		}
	}
	return nil
}

// addFile writes one regular file into the archive. Duplicate names are
// dropped: `git ls-files` can report a path more than once (unmerged entries),
// and a submodule walk can overlap an explicit listing.
func addFile(tw *tar.Writer, name, diskPath string, fi os.FileInfo, seen map[string]bool) error {
	if name == "" || seen[name] {
		return nil
	}
	seen[name] = true

	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return fmt.Errorf("error building tar header for %s: %w", diskPath, err)
	}
	hdr.Name = name
	// Don't leak the local account into the bundle, and keep archives
	// reproducible across machines.
	hdr.Uid, hdr.Gid = 0, 0
	hdr.Uname, hdr.Gname = "", ""

	f, err := os.Open(diskPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("error opening %s: %w", diskPath, err)
	}
	defer func() {
		_ = f.Close()
	}()

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("error writing tar header for %s: %w", name, err)
	}
	// The header size is fixed at stat time, so a file that changes underneath
	// us has to be truncated or padded to match, or the tar writer errors out
	// with ErrWriteTooLong / ErrWriteTooShort and fails the whole scan.
	n, err := io.Copy(tw, io.LimitReader(f, hdr.Size))
	if err != nil {
		return fmt.Errorf("error writing %s to archive: %w", name, err)
	}
	if n < hdr.Size {
		if _, err := io.CopyN(tw, zeroReader{}, hdr.Size-n); err != nil {
			return fmt.Errorf("error padding %s in archive: %w", name, err)
		}
	}
	return nil
}

// withinRoot reports whether p is root or sits beneath it. Both are expected to
// be absolute and symlink-resolved, so the comparison is of real locations
// rather than of the routes taken to reach them.
func withinRoot(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel == "." ||
		(rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
