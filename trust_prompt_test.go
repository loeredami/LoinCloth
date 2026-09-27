package main

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
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

func TestTrustPromptAddToAllowListPersistsExactPath(t *testing.T) {
	withPromptInput(t, "2\nyes")
	state := &State{trustStorePath: filepath.Join(t.TempDir(), "trust.json")}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if !promptExecutableTrust(state, "/usr/bin/example", writer) {
		t.Fatalf("allow-list approval failed: %s", output.String())
	}
	writer.Flush()
	if !state.trustStore.Allows("/usr/bin/example") {
		t.Fatal("approved executable was not added to in-memory trust")
	}
	loaded, err := LoadTrustStore(state.trustStorePath)
	if err != nil {
		t.Fatalf("load persisted store: %v", err)
	}
	if !loaded.Allows("/usr/bin/example") {
		t.Fatal("approved executable was not persisted")
	}
	if !strings.Contains(output.String(), "Add command to allow list") {
		t.Fatalf("prompt did not describe allow-list choice: %q", output.String())
	}
	if !strings.Contains(output.String(), "Persist trust rule") {
		t.Fatalf("prompt did not confirm persistent trust: %q", output.String())
	}
}

func TestTrustPromptDeclinedPersistenceDoesNotTrustOrPersist(t *testing.T) {
	withPromptInput(t, "2\nno")
	path := filepath.Join(t.TempDir(), "trust.json")
	state := &State{trustStorePath: path}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if promptExecutableTrust(state, "/usr/bin/example", writer) {
		t.Fatal("declined persistent trust should deny the invocation")
	}
	writer.Flush()
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("declined persistence modified in-memory trust: %#v", state.trustStore.Entries())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("declined persistence created trust store, stat error: %v", err)
	}
	if !strings.Contains(output.String(), "trust persistence declined") {
		t.Fatalf("expected explicit decline message, got %q", output.String())
	}
}

func TestDeclinedTrustPersistenceDoesNotLaunchCommand(t *testing.T) {
	withPromptInput(t, "2\nno")
	command, state := testCommandHelper(t, "first")
	state.interactiveInput = true
	state.trustStore = TrustStore{}
	state.trustStorePath = filepath.Join(t.TempDir(), "trust.json")

	var output bytes.Buffer
	RunStringTo(state, command, &output)
	if output.Len() != 0 {
		t.Fatalf("command ran after declining persistence: %q", output.String())
	}
	if state.lastExitCode != 126 {
		t.Fatalf("declined command status: got %d, want 126", state.lastExitCode)
	}
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("declined command changed in-memory trust: %#v", state.trustStore.Entries())
	}
	if _, err := os.Stat(state.trustStorePath); !os.IsNotExist(err) {
		t.Fatalf("declined command created trust store, stat error: %v", err)
	}
}

func TestTrustCommandRequiresExplicitConfirmation(t *testing.T) {
	withPromptInput(t, "no")
	state := &State{
		commandSource:    SourceInteractive,
		interactiveInput: true,
		trustStorePath:   filepath.Join(t.TempDir(), "trust.json"),
	}
	if result := HandleStateCommands(state, []string{"!trust", "/usr/bin/example"}); result.HasValue() {
		t.Fatalf("declining confirmation returned error: %v", result.Value())
	}
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("declined trust entry was stored: %#v", state.trustStore.Entries())
	}
	if _, err := os.Stat(state.trustStorePath); !os.IsNotExist(err) {
		t.Fatalf("declined trust created store file, stat error: %v", err)
	}
}

func TestTrustCommandPersistsOnlyAfterConfirmation(t *testing.T) {
	withPromptInput(t, "yes")
	state := &State{
		commandSource:    SourceInteractive,
		interactiveInput: true,
		trustStorePath:   filepath.Join(t.TempDir(), "trust.json"),
	}
	if result := HandleStateCommands(state, []string{"!trust", "/usr/bin/example"}); result.HasValue() {
		t.Fatalf("confirmed trust command returned error: %v", result.Value())
	}
	if !state.trustStore.Allows("/usr/bin/example") {
		t.Fatal("confirmed trust rule was not stored in state")
	}
	loaded, err := LoadTrustStore(state.trustStorePath)
	if err != nil {
		t.Fatalf("load confirmed trust store: %v", err)
	}
	if !loaded.Allows("/usr/bin/example") {
		t.Fatal("confirmed trust rule was not persisted")
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
