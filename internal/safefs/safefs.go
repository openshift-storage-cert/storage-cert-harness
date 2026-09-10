// Package safefs provides path-confined filesystem helpers built on [os.OpenRoot].
//
// The harness accepts many operator-selected paths (plans, backends, reports,
// tool results). Raw os.ReadFile / os.Create calls are flagged by gosec (G304)
// and are vulnerable to path traversal when a relative path contains "..".
// Functions here resolve paths to absolute form and perform I/O through an
// OpenRoot handle so reads and writes cannot escape the intended directory
// (including via symlinks, which OpenRoot rejects).
//
// Choose the entry point by how the path is expressed:
//   - ReadFile / WriteFile — absolute or cwd-relative operator paths
//   - ReadRelative — paths relative to a known root (e.g. "." for CLI overrides)
//   - ReadUnder / WriteUnder — paths already discovered under a results directory
//   - OpenRootDir — create a directory and retain a root for streaming writes
package safefs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// MkdirAll creates dir and every missing parent directory.
//
// Unlike os.MkdirAll, each level is created through OpenRoot.Mkdir on the
// parent, which keeps directory creation inside the resolved tree. If dir
// already exists, MkdirAll returns nil without changing permissions.
func MkdirAll(dir string, perm fs.FileMode) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve directory: %w", err)
	}
	if abs == string(os.PathSeparator) {
		return nil
	}
	if _, err := os.Stat(abs); err == nil {
		return nil
	}
	parent := filepath.Dir(abs)
	if err := MkdirAll(parent, perm); err != nil {
		return err
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return fmt.Errorf("open parent %q: %w", parent, err)
	}
	defer func() { _ = root.Close() }()
	name := filepath.Base(abs)
	if err := root.Mkdir(name, perm); err != nil {
		if _, statErr := root.Stat(name); statErr == nil {
			return nil
		}
		return fmt.Errorf("mkdir %q under %q: %w", name, parent, err)
	}
	return nil
}

// OpenRootDir ensures dir exists (via MkdirAll) and returns an [os.Root] handle
// for it. The caller must close the returned root when finished; any files
// opened through the root should be closed before the root is closed.
func OpenRootDir(dir string, perm fs.FileMode) (*os.Root, error) {
	if err := MkdirAll(dir, perm); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve directory: %w", err)
	}
	return os.OpenRoot(abs)
}

// ReadRelative reads name as a path relative to rootDir.
//
// Use this when the caller supplies a path fragment that must stay inside
// rootDir — for example a --attestation-questions override where ".." must be
// rejected. name is passed directly to [os.Root.ReadFile] and must not contain
// ".." components; absolute names are rejected by OpenRoot.
func ReadRelative(rootDir, name string) ([]byte, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	root, err := os.OpenRoot(absRoot)
	if err != nil {
		return nil, fmt.Errorf("open root %q: %w", rootDir, err)
	}
	defer func() { _ = root.Close() }()
	data, err := root.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ReadFile reads the file at path.
//
// path may be absolute or relative to the process working directory. The parent
// directory is opened as a root and only the basename is read, which blocks
// traversal in the path argument and symlink escape outside the parent.
func ReadFile(path string) ([]byte, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	root, err := os.OpenRoot(filepath.Dir(abs))
	if err != nil {
		return nil, fmt.Errorf("open parent of %q: %w", path, err)
	}
	defer func() { _ = root.Close() }()
	data, err := root.ReadFile(filepath.Base(abs))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ReadUnder reads filePath only when it resolves inside rootDir.
//
// Both arguments may be absolute or relative; they are cleaned and compared
// with filepath.Rel. Use this for tool result collection where finders return
// absolute paths that must be verified to stay under a known results root.
func ReadUnder(rootDir, filePath string) ([]byte, error) {
	rel, err := relUnder(rootDir, filePath)
	if err != nil {
		return nil, err
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}
	root, err := os.OpenRoot(absRoot)
	if err != nil {
		return nil, fmt.Errorf("open root %q: %w", rootDir, err)
	}
	defer func() { _ = root.Close() }()
	return root.ReadFile(rel)
}

// WriteFile writes data to path, creating parent directories as needed.
//
// Parent directories are created with MkdirAll using a mode derived from perm
// (see dirPerm). The file itself is written through OpenRoot on the parent with
// the supplied perm.
func WriteFile(path string, data []byte, perm fs.FileMode) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	if err := MkdirAll(filepath.Dir(abs), dirPerm(perm)); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(abs))
	if err != nil {
		return fmt.Errorf("open parent of %q: %w", path, err)
	}
	defer func() { _ = root.Close() }()
	return root.WriteFile(filepath.Base(abs), data, perm)
}

// WriteWriter buffers fn's output and persists it with WriteFile.
//
// This is a convenience for report generators that already write to an
// io.Writer; the full payload is held in memory before the confined write.
func WriteWriter(path string, perm fs.FileMode, fn func(io.Writer) error) error {
	var buf strings.Builder
	if err := fn(&buf); err != nil {
		return err
	}
	return WriteFile(path, []byte(buf.String()), perm)
}

// WriteUnder writes relPath as a file inside rootDir.
//
// relPath must not escape rootDir (no ".." prefix after cleaning). Intermediate
// subdirectories are created under the root via mkdirUnder. rootDir must
// already exist or be creatable by the caller before writing nested paths.
func WriteUnder(rootDir, relPath string, data []byte, perm fs.FileMode) error {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}
	clean := filepath.Clean(relPath)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("path %q escapes root %q", relPath, rootDir)
	}
	root, err := os.OpenRoot(absRoot)
	if err != nil {
		return fmt.Errorf("open root %q: %w", rootDir, err)
	}
	defer func() { _ = root.Close() }()
	if dir := filepath.Dir(clean); dir != "." {
		if err := mkdirUnder(root, dir, dirPerm(perm)); err != nil {
			return err
		}
	}
	return root.WriteFile(clean, data, perm)
}

// relUnder returns the path of filePath relative to rootDir.
//
// It rejects results that escape rootDir (rel == ".." or starts with "../").
// Both inputs are resolved to absolute paths before comparison.
func relUnder(rootDir, filePath string) (string, error) {
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absFile)
	if err != nil {
		return "", fmt.Errorf("rel path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q escapes root %q", filePath, rootDir)
	}
	return rel, nil
}

// mkdirUnder creates each component of rel inside root.
//
// rel uses slash-separated components (filepath.ToSlash). Existing directories
// are tolerated; any other Mkdir error is returned.
func mkdirUnder(root *os.Root, rel string, perm fs.FileMode) error {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	here := ""
	for _, part := range parts {
		if part == "" || part == "." {
			continue
		}
		next := part
		if here != "" {
			next = here + "/" + part
		}
		if err := root.Mkdir(next, perm); err != nil {
			if _, statErr := root.Stat(next); statErr != nil {
				return fmt.Errorf("mkdir %q: %w", next, err)
			}
		}
		here = next
	}
	return nil
}

// dirPerm picks a directory mode for parent creation when writing a file.
//
// If the file is world-readable (perm & 004), parents get 0755 so shared tool
// containers can traverse the tree; otherwise parents get 0750.
func dirPerm(filePerm fs.FileMode) fs.FileMode {
	if filePerm&0o004 != 0 {
		return 0o755
	}
	return 0o750
}
