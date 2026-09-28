package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestTrustBootstrapAcceptsAllowlistedBasenamesFromDefaultCloth(t *testing.T) {
	state := newInputTestState()
	var output bytes.Buffer
	targets := platformBootstrapTargets()
	if len(targets) == 0 {
		t.Fatal("platform bootstrap allowlist is empty")
	}
	RunStringToSource(state, "!trust-bootstrap "+targets[0], &output, SourceDefaultCloth)
	entries := state.trustStore.Entries()
	if len(entries) != 1 || entries[0].Source != SourcePlatformBootstrap || entries[0].Kind != TrustBasename {
		t.Fatalf("bootstrap trust entry = %#v; output %q", entries, output.String())
	}
}

func TestTrustBootstrapRejectsInteractiveUse(t *testing.T) {
	state := &State{commandSource: SourceInteractive, interactiveInput: true}
	result := HandleStateCommands(state, []string{"!trust-bootstrap", "whoami"})
	if !result.HasValue() || !strings.Contains(result.Value().Error(), "validated default.cloth") {
		t.Fatalf("interactive bootstrap was accepted: %v", result)
	}
}

func TestTrustBootstrapRejectsArbitraryTargets(t *testing.T) {
	state := &State{commandSource: SourceDefaultCloth}
	result := HandleStateCommands(state, []string{"!trust-bootstrap", "totally-unlisted-tool"})
	if !result.HasValue() || !strings.Contains(result.Value().Error(), "allowlist") {
		t.Fatalf("arbitrary bootstrap target was accepted: %v", result)
	}
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("rejected bootstrap modified trust: %#v", state.trustStore.Entries())
	}
}

func TestTrustBootstrapRejectsWorkspaceCommands(t *testing.T) {
	state := &State{commandSource: SourceDefaultCloth}
	result := HandleStateCommands(state, []string{"!trust-bootstrap", "!wear"})
	if !result.HasValue() || !strings.Contains(result.Value().Error(), "workspace commands") {
		t.Fatalf("workspace bootstrap target was accepted: %v", result)
	}
}
