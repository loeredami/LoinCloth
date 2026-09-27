//go:build linux || darwin

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTrustStoreLoadProtectsUnixPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trust.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"entries":[]}`), 0644); err != nil {
		t.Fatalf("write trust store: %v", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("make trust directory permissive: %v", err)
	}
	if _, err := LoadTrustStore(path); err != nil {
		t.Fatalf("load and protect trust store: %v", err)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat trust store directory: %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat trust store file: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("trust store directory mode = %04o, want 0700", dirInfo.Mode().Perm())
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Errorf("trust store file mode = %04o, want 0600", fileInfo.Mode().Perm())
	}
}

func TestTrustStoreSaveCreatesPrivateLocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".loin", "trust.json")
	if err := SaveTrustStore(path, TrustStore{}); err != nil {
		t.Fatalf("save trust store: %v", err)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat trust store directory: %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat trust store file: %v", err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("created trust store directory mode = %04o, want 0700", dirInfo.Mode().Perm())
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Errorf("created trust store file mode = %04o, want 0600", fileInfo.Mode().Perm())
	}
}

func TestTrustStoreLockHasPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".loin", "trust.json")
	if _, err := LoadTrustStore(path); err != nil {
		t.Fatalf("load absent trust store: %v", err)
	}
	lockInfo, err := os.Stat(path + ".lock")
	if err != nil {
		t.Fatalf("stat trust store lock: %v", err)
	}
	if lockInfo.Mode().Perm() != 0600 {
		t.Fatalf("trust store lock mode = %04o, want 0600", lockInfo.Mode().Perm())
	}
}

func TestTrustStoreRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	link := filepath.Join(dir, "trust.json")
	if err := os.WriteFile(target, []byte(`{"version":1,"entries":[]}`), 0600); err != nil {
		t.Fatalf("write trust target: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if _, err := LoadTrustStore(link); err == nil {
		t.Fatal("trust store symlink was accepted")
	}
}
