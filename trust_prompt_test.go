package main

import (
	"bufio"
	"bytes"
	"os"
	"strings"
	"testing"
)

func withPromptInput(t *testing.T, input string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create prompt input pipe: %v", err)
	}
	if _, err := writer.WriteString(input + "\n"); err != nil {
		t.Fatalf("write prompt input: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close prompt input: %v", err)
	}
	previous := os.Stdin
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = previous
		reader.Close()
	})
}

func TestTrustPromptRunOnceDoesNotPersist(t *testing.T) {
	withPromptInput(t, "1")
	state := &State{}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if !promptExecutableTrust(state, "/usr/bin/example", writer) {
		t.Fatal("Run Once should allow this invocation")
	}
	writer.Flush()
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("Run Once persisted trust: %#v", state.trustStore.Entries())
	}
}

func TestTrustPromptTrustsExactPathForCurrentSession(t *testing.T) {
	withPromptInput(t, "2\nyes")
	state := &State{}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if !promptExecutableTrust(state, "/usr/bin/example", writer) {
		t.Fatalf("session trust approval failed: %s", output.String())
	}
	writer.Flush()
	if !state.trustStore.Allows("/usr/bin/example") {
		t.Fatal("approved executable was not trusted for this session")
	}
	state.commandSource = SourceInteractive
	state.interactiveInput = true
	if decision := EvaluateTrust(state.trustStore, "/usr/bin/example", state.commandSource, state.interactiveInput); decision != TrustAllow {
		t.Fatalf("subsequent command was not trusted in this session: %s", decision)
	}
	if strings.Contains(output.String(), "persist") || strings.Contains(output.String(), "allow list") {
		t.Fatalf("prompt implied trust persists beyond this session: %q", output.String())
	}
	if !strings.Contains(output.String(), "Trust for this session") {
		t.Fatalf("prompt did not say trust is session-scoped: %q", output.String())
	}
}

func TestTrustPromptDeclinedSessionTrustDoesNotAuthorize(t *testing.T) {
	withPromptInput(t, "2\nno")
	state := &State{}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if promptExecutableTrust(state, "/usr/bin/example", writer) {
		t.Fatal("declined session trust should deny the invocation")
	}
	writer.Flush()
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("declined session trust modified in-memory rules: %#v", state.trustStore.Entries())
	}
	if !strings.Contains(output.String(), "session trust declined") {
		t.Fatalf("expected explicit decline message, got %q", output.String())
	}
}

func TestDeclinedSessionTrustDoesNotLaunchCommand(t *testing.T) {
	withPromptInput(t, "2\nno")
	command, state := testCommandHelper(t, "first")
	state.interactiveInput = true
	state.trustStore = TrustStore{}

	var output bytes.Buffer
	RunStringTo(state, command, &output)
	if output.Len() != 0 {
		t.Fatalf("command ran after declining persistence: %q", output.String())
	}
	if state.lastExitCode != 126 {
		t.Fatalf("declined command status: got %d, want 126", state.lastExitCode)
	}
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("declined command changed session trust: %#v", state.trustStore.Entries())
	}
}

func TestTrustCommandRequiresExplicitConfirmation(t *testing.T) {
	withPromptInput(t, "no")
	state := &State{
		commandSource:    SourceInteractive,
		interactiveInput: true,
	}
	if result := HandleStateCommands(state, []string{"!trust", "/usr/bin/example"}); result.HasValue() {
		t.Fatalf("declining confirmation returned error: %v", result.Value())
	}
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("declined trust entry was added to the session: %#v", state.trustStore.Entries())
	}
}

func TestTrustCommandAddsSessionRuleOnlyAfterConfirmation(t *testing.T) {
	withPromptInput(t, "yes")
	state := &State{
		commandSource:    SourceInteractive,
		interactiveInput: true,
	}
	if result := HandleStateCommands(state, []string{"!trust", "/usr/bin/example"}); result.HasValue() {
		t.Fatalf("confirmed trust command returned error: %v", result.Value())
	}
	if !state.trustStore.Allows("/usr/bin/example") {
		t.Fatal("confirmed trust rule was not added to the session")
	}
}

func TestTrustCommandRejectsWorkspaceCommandTargets(t *testing.T) {
	state := &State{
		commandSource:    SourceInteractive,
		interactiveInput: true,
	}
	for _, target := range []string{"!wear", "!wear*"} {
		result := HandleStateCommands(state, []string{"!trust", target})
		if !result.HasValue() || !strings.Contains(result.Value().Error(), "workspace commands") {
			t.Fatalf("target %q was not rejected: %v", target, result)
		}
	}
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("workspace trust targets changed in-memory store: %#v", state.trustStore.Entries())
	}
}

func TestSessionTrustDoesNotLoadFromOrWritePersistentStore(t *testing.T) {
	path := trustStoreTestPath(t)
	persisted := TrustStore{}
	persisted.Add(TrustEntry{Rule: "/usr/bin/persisted", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, persisted); err != nil {
		t.Fatalf("seed legacy store: %v", err)
	}

	state := &State{commandSource: SourceInteractive, interactiveInput: true}
	if state.trustStore.Allows("/usr/bin/persisted") {
		t.Fatal("new session inherited trust from disk")
	}
	withPromptInput(t, "yes")
	if result := HandleStateCommands(state, []string{"!trust", "/usr/bin/session"}); result.HasValue() {
		t.Fatalf("session trust command failed: %v", result.Value())
	}

	loaded, err := LoadTrustStore(path)
	if err != nil {
		t.Fatalf("read legacy store after session trust: %v", err)
	}
	if !loaded.Allows("/usr/bin/persisted") || loaded.Allows("/usr/bin/session") {
		t.Fatalf("session trust modified persistent file: %#v", loaded.Entries())
	}
}

func TestUntrustRemovesOnlyCurrentSessionRule(t *testing.T) {
	path := trustStoreTestPath(t)
	persisted := TrustStore{}
	persisted.Add(TrustEntry{Rule: "/usr/bin/example", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, persisted); err != nil {
		t.Fatalf("seed legacy store: %v", err)
	}
	state := &State{commandSource: SourceInteractive, interactiveInput: true}
	state.trustStore.Add(TrustEntry{Rule: "/usr/bin/example", Kind: TrustExactPath, Source: SourceInteractive})
	if result := HandleStateCommands(state, []string{"!untrust", "/usr/bin/example"}); result.HasValue() {
		t.Fatalf("session untrust failed: %v", result.Value())
	}
	if state.trustStore.Allows("/usr/bin/example") {
		t.Fatal("session trust rule was not removed")
	}
	loaded, err := LoadTrustStore(path)
	if err != nil || !loaded.Allows("/usr/bin/example") {
		t.Fatalf("session untrust modified legacy persistent file: entries=%#v err=%v", loaded.Entries(), err)
	}
}

func TestTrustPromptRejectsOtherChoice(t *testing.T) {
	withPromptInput(t, "3")
	state := &State{}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if promptExecutableTrust(state, "/usr/bin/example", writer) {
		t.Fatal("Do not run choice allowed the executable")
	}
	writer.Flush()
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("denial modified trust: %#v", state.trustStore.Entries())
	}
}
