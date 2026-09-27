//go:build windows

package main

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"unsafe"
)

var testSetNamedSecurityInfo = testAdvapi32.NewProc("SetNamedSecurityInfoW")

func trustStoreTestPath(t *testing.T) string {
	t.Helper()
	current, err := user.Current()
	if err != nil {
		t.Fatalf("identify current user: %v", err)
	}
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatalf("create trust store fixture directory: %v", err)
	}

	acl, storage, _ := makeTestDACL(t, []string{current.Uid, "S-1-5-18", "S-1-5-32-544"})
	name, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		t.Fatalf("encode trust store fixture directory: %v", err)
	}
	result, _, _ := testSetNamedSecurityInfo.Call(
		uintptr(unsafe.Pointer(name)),
		seFileObject,
		daclSecurityInfo|0x80000000,
		0,
		0,
		acl,
		0,
	)
	if result != 0 {
		t.Fatalf("protect trust store fixture directory: %v", syscall.Errno(result))
	}
	runtime.KeepAlive(storage)
	if err := validateTrustStorePath(dir, true); err != nil {
		t.Skipf("cannot create a protected trust-store fixture on this Windows runtime: %v", err)
	}
	return filepath.Join(dir, "trust.json")
}
