//go:build darwin || linux

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunSudoCommandPassesArgumentsWithoutShellExpansion(t *testing.T) {
	dir := t.TempDir()
	sudo := filepath.Join(dir, "sudo")
	script := "#!/bin/sh\nprintf '<%s>\\n' \"$@\"\n"
	if err := os.WriteFile(sudo, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	var output bytes.Buffer
	err := runSudoCommand([]string{"/tmp/target with spaces", "literal;$HOME", `quote"value`}, bytes.NewReader(nil), &output, &output)
	if err != nil {
		t.Fatalf("runSudoCommand() error = %v", err)
	}
	want := "<-->\n</tmp/target with spaces>\n<literal;$HOME>\n<quote\"value>\n"
	if got := output.String(); got != want {
		t.Fatalf("sudo arguments output = %q, want %q", got, want)
	}
}

func TestSudoPipelineUsesTargetTrustAndPreservesOutput(t *testing.T) {
	dir := t.TempDir()
	sudo := filepath.Join(dir, "sudo")
	if err := os.WriteFile(sudo, []byte("#!/bin/sh\nprintf '<%s>\\n' \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	state := newInputTestState()
	state.interactiveInput = true
	state.trustStore.Add(TrustEntry{Rule: "/bin/echo", Kind: TrustExactPath, Source: SourceInteractive})
	state.trustStore.Add(TrustEntry{Rule: "/bin/cat", Kind: TrustExactPath, Source: SourceInteractive})
	var output bytes.Buffer
	RunStringToSource(state, "sudo /bin/echo hello | /bin/cat", &output, SourceInteractive)

	if state.lastExitCode != 0 {
		t.Fatalf("pipeline status = %d, output %q", state.lastExitCode, output.String())
	}
	if got, want := output.String(), "<-->\n</bin/echo>\n<hello>\n"; got != want {
		t.Fatalf("pipeline output = %q, want %q", got, want)
	}
}
