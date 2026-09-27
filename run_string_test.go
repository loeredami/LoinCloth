package main

import (
	"bytes"
	"fmt"
	"io"
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
	command, state := testCommandHelper(t, "first")
	state.trustStore = TrustStore{}
	state.interactiveInput = false
	var output bytes.Buffer
	RunStringTo(state, command, &output)
	if output.Len() != 0 {
		t.Fatalf("untrusted command unexpectedly wrote output: %q", output.String())
	}
	if strings.Contains(output.String(), "should-not-run") {
		t.Fatalf("untrusted command appears to have run: %q", output.String())
	}
}

func TestRunStringDeniesBeforeCreatingRedirectOutput(t *testing.T) {
	command, state := testCommandHelper(t, "first")
	state.trustStore = TrustStore{}
	state.interactiveInput = false
	path := filepath.Join(t.TempDir(), "must-not-be-created.txt")
	quotedPath := `"` + path + `"`
	var output bytes.Buffer
	RunStringTo(state, command+` > `+quotedPath, &output)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("untrusted command created redirected output, stat error: %v", err)
	}
}

func TestRunStringAllowsTrustedNonInteractiveCommand(t *testing.T) {
	command, state := testCommandHelper(t, "first")
	state.interactiveInput = false
	var output bytes.Buffer
	RunStringTo(state, command, &output)
	if got, want := output.String(), "first\n"; got != want {
		t.Fatalf("trusted command output: got %q, want %q", got, want)
	}
}

func TestCommandExecutionHelper(t *testing.T) {
	helperIndex := -1
	for i, arg := range os.Args {
		if arg == "--loin-test-helper" {
			helperIndex = i
			break
		}
	}
	if helperIndex < 0 {
		return
	}
	if helperIndex+1 >= len(os.Args) {
		os.Exit(2)
	}

	switch os.Args[helperIndex+1] {
	case "lines":
		_, _ = io.WriteString(os.Stdout, "zulu\nalpha\nbravo\n")
	case "first":
		_, _ = io.WriteString(os.Stdout, "first\n")
	case "second":
		_, _ = io.WriteString(os.Stdout, "second\n")
	case "copy":
		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown test helper mode")
		os.Exit(2)
	}
	os.Exit(0)
}

func testCommandHelper(t *testing.T, mode string) (string, *State) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("get test executable path: %v", err)
	}
	path, err := exec.LookPath(executable)
	if err != nil {
		t.Fatalf("resolve test executable path: %v", err)
	}
	state := &State{
		workspaces: ungo.NewLinkedList[*Workspace](),
		config:     DefaultConfiguration(),
	}
	state.trustStore.Add(TrustEntry{Rule: normalizeExecutablePath(path), Kind: TrustExactPath, Source: SourceInteractive})
	return `"` + path + `" -test.run=TestCommandExecutionHelper -- --loin-test-helper ` + mode, state
}

func TestRunStringPipeline(t *testing.T) {
	helperCommand, state := testCommandHelper(t, "lines")
	filter, err := exec.LookPath("sort")
	if err != nil {
		t.Skipf("required executable %q is unavailable", "sort")
	}
	state.trustStore.Add(TrustEntry{Rule: normalizeExecutablePath(filter), Kind: TrustExactPath, Source: SourceInteractive})

	var output bytes.Buffer
	RunStringTo(state, helperCommand+` | sort`, &output)

	if got, want := output.String(), "alpha\nbravo\nzulu\n"; got != want {
		t.Fatalf("pipeline output: got %q, want %q", got, want)
	}
}

func TestRunStringRedirection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.txt")

	var output bytes.Buffer
	firstCommand, state := testCommandHelper(t, "first")
	secondCommand, _ := testCommandHelper(t, "second")
	quotedPath := `"` + path + `"`
	RunStringTo(state, firstCommand+` > `+quotedPath, &output)
	RunStringTo(state, secondCommand+` >> `+quotedPath, &output)

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
	helperCommand, state := testCommandHelper(t, "copy")
	filter := "grep"
	if is_windows {
		filter = "findstr"
	}
	filterPath, err := exec.LookPath(filter)
	if err != nil {
		t.Skipf("required executable %q is unavailable", filter)
	}
	state.trustStore.Add(TrustEntry{Rule: normalizeExecutablePath(filterPath), Kind: TrustExactPath, Source: SourceInteractive})
	RunStringTo(state, helperCommand+` < "`+inputPath+`" | `+filter+` alpha`, &output)
	if !strings.EqualFold(output.String(), "alpha\n") {
		t.Fatalf("input pipeline output: got %q, want %q", output.String(), "alpha\n")
	}
}
