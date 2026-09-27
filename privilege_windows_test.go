//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestQuoteWindowsArgument(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "plain", want: "plain"},
		{input: "", want: `""`},
		{input: "with spaces", want: `"with spaces"`},
		{input: `embedded"quote`, want: `"embedded\"quote"`},
		{input: `trailing space\`, want: `"trailing space\\"`},
		{input: `slashes\\before"quote`, want: `"slashes\\before\"quote"`},
	}
	for _, test := range tests {
		if got := quoteWindowsArgument(test.input); got != test.want {
			t.Errorf("quoteWindowsArgument(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestWindowsSudoRejectsRedirectionBeforeTruncating(t *testing.T) {
	state := newInputTestState()
	state.interactiveInput = true
	destination := filepath.Join(t.TempDir(), "output.txt")
	if err := os.WriteFile(destination, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	command := PipelineCommand{args: []string{"sudo", "notepad.exe"}, stdoutPath: destination}
	var output bytes.Buffer

	if status := runPipeline(state, []PipelineCommand{command}, &output); status == 0 {
		t.Fatalf("sudo redirection succeeded: %q", output.String())
	}
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep" {
		t.Fatalf("rejected sudo redirection changed existing output: %q", data)
	}
}

func TestWindowsSudoRejectsBufferedStreamsWithoutLaunching(t *testing.T) {
	var output bytes.Buffer
	err := runSudoCommand([]string{"notepad.exe"}, os.Stdin, &output, os.Stderr)
	if err == nil {
		t.Fatal("sudo accepted buffered output and unexpectedly attempted UAC")
	}
}
