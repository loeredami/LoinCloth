//go:build linux || darwin

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func withTrustStoreLock(path string, action func() error) error {
	dir := filepath.Dir(path)
	if err := prepareTrustStoreDirectory(dir); err != nil {
		return err
	}
	lockPath := path + ".lock"
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return fmt.Errorf("open trust store lock: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect trust store lock: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("trust store lock must be a regular file")
	}
	lockStat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(lockStat.Uid) != os.Getuid() {
		return fmt.Errorf("trust store lock is not owned by the current user")
	}
	if err := file.Chmod(0600); err != nil {
		return fmt.Errorf("restrict trust store lock permissions: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock trust store: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return action()
}
