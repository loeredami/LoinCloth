//go:build windows

package main

import "testing"

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

func TestProtectedDACLAllowsOnlyApprovedSIDs(t *testing.T) {
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
