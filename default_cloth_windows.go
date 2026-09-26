//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"unsafe"
)

const (
	fileReadAttributes      = 0x0080
	readControl             = 0x00020000
	fileShareRead           = 0x00000001
	fileShareWrite          = 0x00000002
	fileShareDelete         = 0x00000004
	openExisting            = 3
	fileFlagBackupSemantics = 0x02000000
	fileFlagOpenReparse     = 0x00200000
	fileAttributeDirectory  = 0x00000010
	fileAttributeReparse    = 0x00000400
	ownerSecurityInfo       = 0x00000001
	daclSecurityInfo        = 0x00000004
	seFileObject            = 1
	aclSizeInformation      = 2
	accessAllowedACEType    = 0
	accessDeniedACEType     = 1
	accessAuditACEType      = 2
	accessAlarmACEType      = 3
	aclACEHeaderSize        = 4
	inheritOnlyACE          = 0x08
)

var (
	kernel32ACL              = syscall.NewLazyDLL("kernel32.dll")
	advapi32ACL              = syscall.NewLazyDLL("advapi32.dll")
	procGetSecurityInfo      = advapi32ACL.NewProc("GetSecurityInfo")
	procGetAclInformation    = advapi32ACL.NewProc("GetAclInformation")
	procGetAce               = advapi32ACL.NewProc("GetAce")
	procEqualSid             = advapi32ACL.NewProc("EqualSid")
	procIsValidSID           = advapi32ACL.NewProc("IsValidSid")
	procGetLengthSID         = advapi32ACL.NewProc("GetLengthSid")
	procConvertSIDToString   = advapi32ACL.NewProc("ConvertSidToStringSidW")
	procConvertStringSID     = advapi32ACL.NewProc("ConvertStringSidToSidW")
	procLocalFreeACL         = kernel32ACL.NewProc("LocalFree")
	procGetSecurityDescDACL  = advapi32ACL.NewProc("GetSecurityDescriptorDacl")
	procGetSecurityDescOwner = advapi32ACL.NewProc("GetSecurityDescriptorOwner")
)

type aclSizeInfo struct {
	AceCount      uint32
	AclBytesInUse uint32
	AclBytesFree  uint32
}

func readProtectedDefaultCloth(path string) ([]byte, error) {
	current, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("identify current Windows user: %w", err)
	}
	currentSID, err := sidFromString(current.Uid)
	if err != nil {
		return nil, fmt.Errorf("resolve current Windows user SID: %w", err)
	}
	defer freeLocalMemory(currentSID)

	trustedSIDs := []uintptr{uintptr(currentSID)}
	for _, sidText := range []string{"S-1-5-18", "S-1-5-32-544"} {
		sid, err := sidFromString(sidText)
		if err != nil {
			return nil, fmt.Errorf("resolve trusted Windows SID %s: %w", sidText, err)
		}
		trustedSIDs = append(trustedSIDs, uintptr(sid))
		defer freeLocalMemory(sid)
	}

	dir, err := openForACL(filepath.Dir(path), fileReadAttributes|readControl, fileShareRead|fileShareWrite|fileShareDelete, fileFlagBackupSemantics|fileFlagOpenReparse)
	if err != nil {
		return nil, fmt.Errorf("open default.cloth directory for security validation: %w", err)
	}
	defer dir.Close()
	if err := validateProtectedHandle(syscall.Handle(dir.Fd()), true, uintptr(currentSID), trustedSIDs); err != nil {
		return nil, fmt.Errorf("default.cloth directory is not protected: %w", err)
	}

	file, err := openForACL(path, syscall.GENERIC_READ|readControl, fileShareRead, fileFlagOpenReparse)
	if err != nil {
		return nil, fmt.Errorf("open default.cloth without following reparse points: %w", err)
	}
	defer file.Close()
	if err := validateProtectedHandle(syscall.Handle(file.Fd()), false, uintptr(currentSID), trustedSIDs); err != nil {
		return nil, fmt.Errorf("default.cloth is not protected: %w", err)
	}
	return io.ReadAll(file)
}

func openForACL(path string, access, share, flags uint32) (*os.File, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(name, access, share, nil, openExisting, flags, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}

func validateProtectedHandle(handle syscall.Handle, expectDirectory bool, ownerSID uintptr, trustedSIDs []uintptr) error {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil {
		return fmt.Errorf("inspect file handle: %w", err)
	}
	isDirectory := info.FileAttributes&fileAttributeDirectory != 0
	if isDirectory != expectDirectory {
		return fmt.Errorf("unexpected file type")
	}
	if info.FileAttributes&fileAttributeReparse != 0 {
		return fmt.Errorf("reparse points are not allowed")
	}

	var owner, dacl, descriptor uintptr
	result, _, _ := procGetSecurityInfo.Call(
		uintptr(handle), seFileObject, ownerSecurityInfo|daclSecurityInfo,
		uintptr(unsafe.Pointer(&owner)), 0, uintptr(unsafe.Pointer(&dacl)), 0,
		uintptr(unsafe.Pointer(&descriptor)),
	)
	if result != 0 {
		return syscall.Errno(result)
	}
	defer freeLocalMemory(syscall.Handle(descriptor))

	var descriptorOwner uintptr
	var ownerDefaulted int32
	ok, _, callErr := procGetSecurityDescOwner.Call(descriptor, uintptr(unsafe.Pointer(&descriptorOwner)), uintptr(unsafe.Pointer(&ownerDefaulted)))
	if ok == 0 {
		return fmt.Errorf("read security descriptor owner: %w", callErr)
	}
	if !sidEqual(descriptorOwner, ownerSID) {
		return fmt.Errorf("owner is not the current user")
	}

	var daclPresent, daclDefaulted int32
	var descriptorDACL uintptr
	ok, _, callErr = procGetSecurityDescDACL.Call(descriptor, uintptr(unsafe.Pointer(&daclPresent)), uintptr(unsafe.Pointer(&descriptorDACL)), uintptr(unsafe.Pointer(&daclDefaulted)))
	if ok == 0 {
		return fmt.Errorf("read security descriptor DACL: %w", callErr)
	}
	if daclPresent == 0 || descriptorDACL == 0 {
		return fmt.Errorf("missing or unrestricted DACL")
	}

	var aclInfo aclSizeInfo
	ok, _, callErr = procGetAclInformation.Call(descriptorDACL, uintptr(unsafe.Pointer(&aclInfo)), unsafe.Sizeof(aclInfo), aclSizeInformation)
	if ok == 0 {
		return fmt.Errorf("read DACL information: %w", callErr)
	}
	for index := uint32(0); index < aclInfo.AceCount; index++ {
		var ace uintptr
		ok, _, callErr = procGetAce.Call(descriptorDACL, uintptr(index), uintptr(unsafe.Pointer(&ace)))
		if ok == 0 {
			return fmt.Errorf("read DACL entry: %w", callErr)
		}
		aceType := *(*byte)(unsafe.Pointer(ace))
		aceFlags := *(*byte)(unsafe.Pointer(ace + 1))
		allowedACE, appliesToObject, err := protectedACEPolicy(aceType, aceFlags)
		if err != nil {
			return err
		}
		if !allowedACE || !appliesToObject {
			continue
		}
		aceSize := *(*uint16)(unsafe.Pointer(ace + 2))
		const sidOffset = aclACEHeaderSize + 4
		if uint32(aceSize) > aclInfo.AclBytesInUse || int(aceSize) < sidOffset+8 {
			return fmt.Errorf("malformed allowed DACL entry")
		}
		aceSID := ace + sidOffset
		valid, _, _ := procIsValidSID.Call(aceSID)
		if valid == 0 {
			return fmt.Errorf("malformed SID in allowed DACL entry")
		}
		sidLength, _, _ := procGetLengthSID.Call(aceSID)
		if sidLength > uintptr(int(aceSize)-sidOffset) {
			return fmt.Errorf("SID exceeds its DACL entry")
		}
		if !isTrustedSID(aceSID, trustedSIDs) {
			sidText := sidToString(aceSID)
			return fmt.Errorf("DACL grants access to an untrusted SID %s", sidText)
		}
	}
	return nil
}

func sidFromString(value string) (syscall.Handle, error) {
	text, err := syscall.UTF16PtrFromString(value)
	if err != nil {
		return 0, err
	}
	var sid uintptr
	ok, _, callErr := procConvertStringSID.Call(uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(&sid)))
	if ok == 0 {
		if callErr != syscall.Errno(0) {
			return 0, callErr
		}
		return 0, fmt.Errorf("ConvertStringSidToSidW failed")
	}
	return syscall.Handle(sid), nil
}

func protectedACEPolicy(aceType, aceFlags byte) (allowedACE, appliesToObject bool, err error) {
	switch aceType {
	case accessDeniedACEType, accessAuditACEType, accessAlarmACEType:
		return false, false, nil
	case accessAllowedACEType:
		return true, aceFlags&inheritOnlyACE == 0, nil
	default:
		return false, false, fmt.Errorf("unsupported DACL entry type %d", aceType)
	}
}

func isTrustedSID(sid uintptr, trustedSIDs []uintptr) bool {
	for _, trustedSID := range trustedSIDs {
		if sidEqual(sid, trustedSID) {
			return true
		}
	}
	return false
}

func sidToString(sid uintptr) string {
	var text uintptr
	ok, _, _ := procConvertSIDToString.Call(sid, uintptr(unsafe.Pointer(&text)))
	if ok == 0 || text == 0 {
		return "<unprintable SID>"
	}
	defer freeLocalMemory(syscall.Handle(text))
	return syscall.UTF16ToString((*[1 << 20]uint16)(unsafe.Pointer(text))[:])
}

func sidEqual(left, right uintptr) bool {
	ok, _, _ := procEqualSid.Call(left, right)
	return ok != 0
}

func freeLocalMemory(handle syscall.Handle) {
	if handle != 0 {
		procLocalFreeACL.Call(uintptr(handle))
	}
}

func readDefaultCloth(path string) ([]byte, error) {
	return os.ReadFile(path)
}
