//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	trustStoreLockKernel32 = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx         = trustStoreLockKernel32.NewProc("LockFileEx")
	procUnlockFileEx       = trustStoreLockKernel32.NewProc("UnlockFileEx")
)

type trustStoreOverlapped struct {
	Internal     uintptr
	InternalHigh uintptr
	Offset       uint32
	OffsetHigh   uint32
	HEvent       syscall.Handle
}

func withTrustStoreLock(path string, action func() error) error {
	dir := filepath.Dir(path)
	if err := prepareTrustStoreDirectory(dir); err != nil {
		return err
	}
	lockPath := path + ".lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open trust store lock: %w", err)
	}
	defer file.Close()
	if err := validateTrustStorePath(lockPath, false); err != nil {
		return fmt.Errorf("trust store lock is not protected: %w", err)
	}
	var overlapped trustStoreOverlapped
	ok, _, callErr := procLockFileEx.Call(
		file.Fd(),
		0x00000002,
		0,
		1,
		0,
		uintptr(unsafe.Pointer(&overlapped)),
	)
	if ok == 0 {
		return fmt.Errorf("lock trust store: %w", callErr)
	}
	defer procUnlockFileEx.Call(file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	return action()
}
