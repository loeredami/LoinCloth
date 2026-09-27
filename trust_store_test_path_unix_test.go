//go:build linux || darwin

package main

import (
	"path/filepath"
	"testing"
)

func trustStoreTestPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "trust.json")
}
