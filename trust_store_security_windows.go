//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
)

func trustStoreSIDs() (uintptr, []uintptr, func(), error) {
	current, err := user.Current()
	if err != nil {
		return 0, nil, nil, fmt.Errorf("identify current Windows user: %w", err)
	}
	currentSID, err := sidFromString(current.Uid)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("resolve current Windows user SID: %w", err)
	}
	trustedSIDs := []uintptr{uintptr(currentSID)}
	sids := []syscall.Handle{currentSID}
	for _, sidText := range []string{"S-1-5-18", "S-1-5-32-544"} {
		sid, err := sidFromString(sidText)
		if err != nil {
			for _, allocatedSID := range sids {
				freeLocalMemory(allocatedSID)
			}
			return 0, nil, nil, fmt.Errorf("resolve trusted Windows SID %s: %w", sidText, err)
		}
		sids = append(sids, sid)
		trustedSIDs = append(trustedSIDs, uintptr(sid))
	}
	release := func() {
		for _, sid := range sids {
			freeLocalMemory(sid)
		}
	}
	return uintptr(currentSID), trustedSIDs, release, nil
}

func validateTrustStorePath(path string, directory bool) error {
	ownerSID, trustedSIDs, release, err := trustStoreSIDs()
	if err != nil {
		return err
	}
	defer release()
	access := uint32(fileReadAttributes | readControl)
	flags := uint32(fileFlagOpenReparse)
	if directory {
		flags |= fileFlagBackupSemantics
	} else {
		access |= syscall.GENERIC_READ
	}
	handle, err := openForACL(path, access, fileShareRead|fileShareWrite|fileShareDelete, flags)
	if err != nil {
		return err
	}
	defer handle.Close()
	return validateProtectedHandle(syscall.Handle(handle.Fd()), directory, ownerSID, trustedSIDs)
}

func prepareTrustStoreDirectory(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return fmt.Errorf("create trust store directory: %w", err)
	}
	return validateTrustStoreDirectory(path)
}

func validateTrustStoreDirectory(path string) error {
	if err := validateTrustStorePath(path, true); err != nil {
		return fmt.Errorf("trust store directory is not protected: %w", err)
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
	ownerSID, trustedSIDs, release, err := trustStoreSIDs()
	if err != nil {
		return nil, err
	}
	defer release()
	file, err := openForACL(path, syscall.GENERIC_READ|readControl, fileShareRead, fileFlagOpenReparse)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("open trust store securely: %w", err)
	}
	defer file.Close()
	if err := validateProtectedHandle(syscall.Handle(file.Fd()), false, ownerSID, trustedSIDs); err != nil {
		return nil, fmt.Errorf("trust store file is not protected: %w", err)
	}
	return io.ReadAll(file)
}

func validateTrustStoreReplacement(path string) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect trust store destination: %w", err)
	}
	if err := validateTrustStorePath(path, false); err != nil {
		return fmt.Errorf("trust store destination is not protected: %w", err)
	}
	return nil
}
