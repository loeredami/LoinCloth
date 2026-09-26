//go:build linux || darwin

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func readProtectedDefaultCloth(path string) ([]byte, error) {
	info, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("inspect config directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("config directory must be a real directory")
	}
	dirStat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(dirStat.Uid) != os.Getuid() {
		return nil, fmt.Errorf("config directory is not owned by the current user")
	}
	if info.Mode().Perm()&0077 != 0 {
		if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
			return nil, fmt.Errorf("restrict config directory permissions: %w", err)
		}
		info, err = os.Lstat(filepath.Dir(path))
		if err != nil || info.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("config directory permissions are not private")
		}
	}

	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open default cloth without following symlinks: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect default cloth: %w", err)
	}
	if !fileInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("default cloth must be a regular file")
	}
	fileStat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok || int(fileStat.Uid) != os.Getuid() {
		return nil, fmt.Errorf("default cloth is not owned by the current user")
	}
	if fileInfo.Mode().Perm()&0077 != 0 {
		if err := file.Chmod(0600); err != nil {
			return nil, fmt.Errorf("restrict default cloth permissions: %w", err)
		}
	}
	return io.ReadAll(file)
}

func readDefaultCloth(path string) ([]byte, error) {
	return os.ReadFile(path)
}
