// Package store handles local persistence.
package store

import (
	"os"
	"path/filepath"
)

// SessionPath returns the canonical path for the gotd session file. It lives
// under the user's config dir so it works cross-platform without changing
// callsites.
func SessionPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		p := filepath.Join(dir, "tergram", "session.json")
		_ = os.MkdirAll(filepath.Dir(p), 0o700)
		return p
	}
	return "session.json"
}
