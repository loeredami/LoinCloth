//go:build windows

package main

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unsafe"
)

const (
	testACLRevision              = 2
	testDACLInformation          = 0x00000004
	testProtectedDACLInformation = 0x80000000
	testWriteDAC                 = 0x00040000
	testFileAllAccess            = 0x001F01FF
	testACLHeaderSize            = 8
	testACEHeaderAndMaskSize     = 8
)

var (
	testAdvapi32            = syscall.NewLazyDLL("advapi32.dll")
	testInitializeACL       = testAdvapi32.NewProc("InitializeAcl")
	testAddAccessAllowedACE = testAdvapi32.NewProc("AddAccessAllowedAce")
	testSetSecurityInfo     = testAdvapi32.NewProc("SetSecurityInfo")
)

func TestProtectedACEPolicy(t *testing.T) {
	tests := []struct {
		name        string
		typ, flags  byte
		wantAllowed bool
		wantApplies bool
		wantError   bool
	}{
		{name: "allow ACE applies", typ: accessAllowedACEType, wantAllowed: true, wantApplies: true},
		{name: "inherit-only allow does not apply", typ: accessAllowedACEType, flags: inheritOnlyACE, wantAllowed: true},
		{name: "deny ACE", typ: accessDeniedACEType},
		{name: "audit ACE", typ: accessAuditACEType},
		{name: "alarm ACE", typ: accessAlarmACEType},
		{name: "unknown ACE fails closed", typ: 0xff, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			allowed, applies, err := protectedACEPolicy(test.typ, test.flags)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError %t", err, test.wantError)
			}
			if allowed != test.wantAllowed || applies != test.wantApplies {
				t.Fatalf("policy = (%t, %t), want (%t, %t)", allowed, applies, test.wantAllowed, test.wantApplies)
			}
		})
	}
}

func TestIsTrustedSID(t *testing.T) {
	owner, err := sidFromString("S-1-5-21-1000")
	if err != nil {
		t.Fatalf("create owner SID: %v", err)
	}
	defer freeLocalMemory(owner)
	other, err := sidFromString("S-1-5-21-2000")
	if err != nil {
		t.Fatalf("create other SID: %v", err)
	}
	defer freeLocalMemory(other)

	if !isTrustedSID(uintptr(owner), []uintptr{uintptr(owner)}) {
		t.Fatal("current-user SID should be trusted")
	}
	if isTrustedSID(uintptr(other), []uintptr{uintptr(owner)}) {
		t.Fatal("different user SID must not be trusted")
	}
}

func TestReadProtectedDefaultClothAcceptsPrivateDACL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.cloth")
	if err := os.WriteFile(path, []byte("!set secure yes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	setTestProtectedDACL(t, dir, true, false)
	setTestProtectedDACL(t, path, false, false)

	got, err := readProtectedDefaultCloth(path)
	if err != nil {
		if strings.Contains(err.Error(), "S-1-1-0") {
			t.Skipf("runtime did not apply the restrictive DACL to the test directory: %v", err)
		}
		t.Fatalf("private default.cloth was rejected: %v", err)
	}
	if string(got) != "!set secure yes\n" {
		t.Fatalf("unexpected contents %q", got)
	}
}

func TestReadProtectedDefaultClothRejectsEveryoneDACL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.cloth")
	if err := os.WriteFile(path, []byte("!set untrusted yes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	setTestProtectedDACL(t, dir, true, true)
	setTestProtectedDACL(t, path, false, false)

	if _, err := readProtectedDefaultCloth(path); err == nil {
		t.Fatal("default.cloth with an Everyone DACL was accepted")
	} else if !strings.Contains(err.Error(), "S-1-1-0") {
		t.Fatalf("rejection did not identify the Everyone SID: %v", err)
	}
}

func setTestProtectedDACL(t *testing.T, path string, isDirectory, includeEveryone bool) {
	t.Helper()
	current, err := user.Current()
	if err != nil {
		t.Fatalf("identify current user: %v", err)
	}
	sidStrings := []string{current.Uid, "S-1-5-18", "S-1-5-32-544"}
	if includeEveryone {
		sidStrings = append(sidStrings, "S-1-1-0")
	}

	sids := make([]syscall.Handle, 0, len(sidStrings))
	defer func() {
		for _, sid := range sids {
			freeLocalMemory(sid)
		}
	}()
	aclLength := testACLHeaderSize
	for _, sidString := range sidStrings {
		sid, err := sidFromString(sidString)
		if err != nil {
			t.Fatalf("convert SID %q: %v", sidString, err)
		}
		sids = append(sids, sid)
		length, _, _ := procGetLengthSID.Call(uintptr(sid))
		aclLength += testACEHeaderAndMaskSize + int(length)
	}

	aclStorage := make([]uint32, (aclLength+3)/4)
	acl := uintptr(unsafe.Pointer(&aclStorage[0]))
	ok, _, callErr := testInitializeACL.Call(acl, uintptr(aclLength), testACLRevision)
	if ok == 0 {
		t.Fatalf("initialize test ACL: %v", callErr)
	}
	for _, sid := range sids {
		ok, _, callErr = testAddAccessAllowedACE.Call(acl, testACLRevision, testFileAllAccess, uintptr(sid))
		if ok == 0 {
			t.Fatalf("add allowed ACE: %v", callErr)
		}
	}

	flags := uint32(fileFlagOpenReparse)
	if isDirectory {
		flags |= fileFlagBackupSemantics
	}
	handle, err := openForACL(path, testWriteDAC|readControl|fileReadAttributes, fileShareRead|fileShareWrite|fileShareDelete, flags)
	if err != nil {
		t.Fatalf("open %q to set test DACL: %v", path, err)
	}
	defer handle.Close()

	securityInformation := uintptr(testDACLInformation | testProtectedDACLInformation)
	result, _, _ := testSetSecurityInfo.Call(uintptr(handle.Fd()), seFileObject, securityInformation, 0, 0, acl, 0, 0)
	if result != 0 {
		t.Fatalf("set test DACL: %v", syscall.Errno(result))
	}
	runtime.KeepAlive(aclStorage)
}
