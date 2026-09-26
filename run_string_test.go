package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loeredami/ungo"
)

func testTrustedState(t *testing.T, executables ...string) *State {
	t.Helper()
	state := &State{
		workspaces: ungo.NewLinkedList[*Workspace](),
		config:     DefaultConfiguration(),
	}
	for _, executable := range executables {
		path, err := exec.LookPath(executable)
		if err != nil {
			t.Skipf("required executable %q is unavailable", executable)
		}
		state.trustStore.Add(TrustEntry{Rule: normalizeExecutablePath(path), Kind: TrustExactPath, Source: SourceInteractive})
	}
	return state
}

func TestRunStringRejectsTrustManagementFromPipedInput(t *testing.T) {
	state := &State{
		workspaces:       ungo.NewLinkedList[*Workspace](),
		config:           DefaultConfiguration(),
		interactiveInput: false,
	}
	var output bytes.Buffer
	RunStringToSource(state, `!trust example`, &output, SourceNonInteractive)
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("piped !trust modified the trust store: %#v", state.trustStore.Entries())
	}
	if !strings.Contains(output.String(), "requires direct interactive input") {
		t.Fatalf("expected direct-input restriction, got %q", output.String())
	}
}

func TestRunStringBlocksUntrustedNonInteractiveCommand(t *testing.T) {
	if is_windows {
		t.Skip("uses Unix printf")
	}
	state := &State{
		workspaces:       ungo.NewLinkedList[*Workspace](),
		config:           DefaultConfiguration(),
		interactiveInput: false,
	}
	var output bytes.Buffer
	RunStringTo(state, `printf should-not-run`, &output)
	if output.Len() != 0 {
		t.Fatalf("untrusted command unexpectedly wrote output: %q", output.String())
	}
	if strings.Contains(output.String(), "should-not-run") {
		t.Fatalf("untrusted command appears to have run: %q", output.String())
	}
}

func TestRunStringDeniesBeforeCreatingRedirectOutput(t *testing.T) {
	if is_windows {
		t.Skip("uses Unix printf")
	}
	state := &State{
		workspaces: ungo.NewLinkedList[*Workspace](),
		config:     DefaultConfiguration(),
	}
	path := filepath.Join(t.TempDir(), "must-not-be-created.txt")
	var output bytes.Buffer
	RunStringTo(state, `printf should-not-run > `+path, &output)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("untrusted command created redirected output, stat error: %v", err)
	}
}

func TestRunStringAllowsTrustedNonInteractiveCommand(t *testing.T) {
	if is_windows {
		t.Skip("uses Unix printf")
	}
	state := testTrustedState(t, "printf")
	var output bytes.Buffer
	RunStringTo(state, `printf trusted-run`, &output)
	if got, want := output.String(), "trusted-run"; got != want {
		t.Fatalf("trusted command output: got %q, want %q", got, want)
	}
}

func TestRunStringPipeline(t *testing.T) {
	var output bytes.Buffer
	RunStringTo(testTrustedState(t, "printf", "sort"), `printf "zulu\nalpha\nbravo\n" | sort`, &output)

	if got, want := output.String(), "alpha\nbravo\nzulu\n"; got != want {
		t.Fatalf("pipeline output: got %q, want %q", got, want)
	}
}

func TestRunStringRedirection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.txt")

	var output bytes.Buffer
	state := testTrustedState(t, "printf")
	RunStringTo(state, `printf "first\n" > `+path, &output)
	RunStringTo(state, `printf "second\n" >> `+path, &output)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read redirected output: %v", err)
	}
	if got, want := string(data), "first\nsecond\n"; got != want {
		t.Fatalf("redirected output: got %q, want %q", got, want)
	}
}

func TestRunStringInputPipeline(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputPath, []byte("zulu\nalpha\nbravo\n"), 0600); err != nil {
		t.Fatalf("write input: %v", err)
	}

	var output bytes.Buffer
	RunStringTo(testTrustedState(t, "cat", "grep"), `cat < `+inputPath+` | grep alpha`, &output)
	if !strings.EqualFold(output.String(), "alpha\n") {
		t.Fatalf("input pipeline output: got %q, want %q", output.String(), "alpha\n")
	}
}
