package main

import (
	"bytes"
	"errors"
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

func TestRunStringTracksSingleCommandExitStatus(t *testing.T) {
	command, state := testCommandHelper(t, "exit-seven")
	state.interactiveInput = false
	var output bytes.Buffer
	RunStringTo(state, command, &output)
	if state.lastExitCode != 7 {
		t.Fatalf("last exit code: got %d, want 7; output %q", state.lastExitCode, output.String())
	}

	output.Reset()
	writeLastStatus(state, &output)
	if got, want := output.String(), "7\n"; got != want {
		t.Fatalf("last-status output: got %q, want %q", got, want)
	}
}

func TestRunStringAssignsFailureStatuses(t *testing.T) {
	command, state := testCommandHelper(t, "exit-seven")
	state.interactiveInput = false
	state.trustStore = TrustStore{}

	RunStringTo(state, command, &bytes.Buffer{})
	if state.lastExitCode != 126 {
		t.Fatalf("denied command status: got %d, want 126", state.lastExitCode)
	}

	RunStringTo(state, "loin-command-that-does-not-exist", &bytes.Buffer{})
	if state.lastExitCode != 127 {
		t.Fatalf("missing command status: got %d, want 127", state.lastExitCode)
	}

	RunStringTo(state, `echo one || echo two`, &bytes.Buffer{})
	if state.lastExitCode != 2 {
		t.Fatalf("parse error status: got %d, want 2", state.lastExitCode)
	}
}

func TestRunStringPipelineStatusUsesFinalStage(t *testing.T) {
	upstreamCommand, state := testCommandHelper(t, "write-exit-seven")
	downstreamCommand, _ := testCommandHelper(t, "copy-and-write-exit-zero")
	state.interactiveInput = false
	var output bytes.Buffer
	RunStringTo(state, upstreamCommand+` | `+downstreamCommand, &output)
	if state.lastExitCode != 0 {
		t.Fatalf("pipeline exit code: got %d, want final-stage status 0; output %q", state.lastExitCode, output.String())
	}
	if got, want := output.String(), "upstream\ndownstream\n"; got != want {
		t.Fatalf("pipeline output order: got %q, want %q", got, want)
	}

	failingCommand, _ := testCommandHelper(t, "exit-seven")
	output.Reset()
	RunStringTo(state, downstreamCommand+` | `+failingCommand, &output)
	if state.lastExitCode != 7 {
		t.Fatalf("pipeline final-stage exit code: got %d, want 7", state.lastExitCode)
	}
}

func TestBlankInputPreservesLastExitStatus(t *testing.T) {
	state := &State{lastExitCode: 7}
	RunStringTo(state, " \t ", &bytes.Buffer{})
	if state.lastExitCode != 7 {
		t.Fatalf("blank input changed last status to %d", state.lastExitCode)
	}
}

func TestRunStringReportsInternalCommandIOFailure(t *testing.T) {
	state := newInputTestState()
	var output bytes.Buffer
	RunStringTo(state, `ls "/loin-cloth-path-that-does-not-exist"`, &output)
	if state.lastExitCode != 1 {
		t.Fatalf("internal I/O failure status: got %d, want 1; output %q", state.lastExitCode, output.String())
	}
	if !strings.Contains(output.String(), "Error:") {
		t.Fatalf("expected ls error output, got %q", output.String())
	}
}

func TestBufferedPipelineStopsAtFirstFailedStage(t *testing.T) {
	state := newInputTestState()
	downstreamRan := false
	RegisterCmd("!pipeline-status-fail", func(state *State, command []string) ungo.Optional[error] {
		return ungo.Some(errors.New("intentional stage failure"))
	})
	RegisterCmd("!pipeline-status-should-not-run", func(state *State, command []string) ungo.Optional[error] {
		downstreamRan = true
		return ungo.None[error]()
	})

	var output bytes.Buffer
	RunStringTo(state, "!pipeline-status-fail | !pipeline-status-should-not-run", &output)
	if downstreamRan {
		t.Fatal("buffered pipeline ran a stage after failure")
	}
	if state.lastExitCode != 1 {
		t.Fatalf("buffered pipeline status: got %d, want 1", state.lastExitCode)
	}
	if !strings.Contains(output.String(), "intentional stage failure") {
		t.Fatalf("failed stage error missing from output: %q", output.String())
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
	case "exit-seven":
		os.Exit(7)
	case "write-exit-seven":
		_, _ = io.WriteString(os.Stdout, "upstream\n")
		os.Exit(7)
	case "write-exit-zero":
		_, _ = io.WriteString(os.Stdout, "downstream\n")
	case "copy-and-write-exit-zero":
		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			os.Exit(1)
		}
		_, _ = io.WriteString(os.Stdout, "downstream\n")
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
