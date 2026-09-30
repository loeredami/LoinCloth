//go:build linux || darwin

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadProtectedDefaultClothAcceptsPrivateOwnedFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "default.cloth")
	if err := os.WriteFile(path, []byte("!set sample value\n"), 0600); err != nil {
		t.Fatal(err)
	}

	got, err := readProtectedDefaultCloth(path)
	if err != nil {
		t.Fatalf("read protected default.cloth: %v", err)
	}
	if string(got) != "!set sample value\n" {
		t.Fatalf("unexpected contents %q", got)
	}
}

func TestReadProtectedDefaultClothTightensPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.cloth")
	if err := os.WriteFile(path, []byte("config"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}

	if _, err := readProtectedDefaultCloth(path); err != nil {
		t.Fatalf("expected owned permissions to be tightened: %v", err)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0700 {
		t.Errorf("directory permissions = %04o, want 0700", dirInfo.Mode().Perm())
	}
	if fileInfo.Mode().Perm() != 0600 {
		t.Errorf("file permissions = %04o, want 0600", fileInfo.Mode().Perm())
	}
}

func TestReadProtectedDefaultClothRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.cloth")
	link := filepath.Join(dir, "default.cloth")
	if err := os.WriteFile(target, []byte("config"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if _, err := readProtectedDefaultCloth(link); err == nil {
		t.Fatal("expected symlink to be rejected")
	}
}

func TestReadProtectedDefaultClothRejectsNonRegularFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := readProtectedDefaultCloth(dir); err == nil {
		t.Fatal("expected directory to be rejected as default.cloth")
	}
}
