// Package artifacts abstracts where logs and raw tool output are stored during a
// run. The skeleton ships a local filesystem store; object-store backends can
// be added behind the same interface. See decisions/0003.
package artifacts

import (
	"os"
	"path/filepath"
)

// Store persists run artifacts.
type Store interface {
	Put(name string, data []byte) (ref string, err error)
}

// FSStore writes artifacts under a base directory.
type FSStore struct {
	Base string
}

// Put writes data to Base/name, creating parent directories.
func (s FSStore) Put(name string, data []byte) (string, error) {
	p := filepath.Join(s.Base, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil { // #nosec G301 -- artifacts are shared with the tool container.
		return "", err
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return "", err
	}
	return p, nil
}
