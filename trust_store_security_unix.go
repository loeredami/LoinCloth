//go:build linux || darwin

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func prepareTrustStoreDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return fmt.Errorf("create trust store directory: %w", err)
	}
	return validateTrustStoreDirectory(path)
}

func validateTrustStoreDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect trust store directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("trust store directory must be a real directory")
	}
	dirStat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(dirStat.Uid) != os.Getuid() {
		return fmt.Errorf("trust store directory is not owned by the current user")
	}
	if info.Mode().Perm()&0077 != 0 {
		if err := os.Chmod(path, 0700); err != nil {
			return fmt.Errorf("restrict trust store directory permissions: %w", err)
		}
		info, err = os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("trust store directory permissions are not private")
		}
	}
	return nil
}

func readTrustStoreFile(path string) ([]byte, error) {
	if err := validateTrustStoreDirectory(filepath.Dir(path)); err != nil {
		if _, statErr := os.Lstat(filepath.Dir(path)); os.IsNotExist(statErr) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect trust store file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("trust store must be a regular file")
	}
	fileStat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(fileStat.Uid) != os.Getuid() {
		return nil, fmt.Errorf("trust store file is not owned by the current user")
	}
	if info.Mode().Perm()&0077 != 0 {
		if err := file.Chmod(0600); err != nil {
			return nil, fmt.Errorf("restrict trust store file permissions: %w", err)
		}
		info, err = file.Stat()
		if err != nil || info.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("trust store file permissions are not private")
		}
	}
	return io.ReadAll(file)
}

func validateTrustStoreReplacement(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect trust store destination: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("trust store destination must be a regular, non-symlink file")
	}
	fileStat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(fileStat.Uid) != os.Getuid() {
		return fmt.Errorf("trust store destination is not owned by the current user")
	}
	if info.Mode().Perm()&0077 != 0 {
		if err := os.Chmod(path, 0600); err != nil {
			return fmt.Errorf("restrict trust store destination permissions: %w", err)
		}
	}
	return nil
}
