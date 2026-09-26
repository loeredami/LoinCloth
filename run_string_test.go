package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunStringPipeline(t *testing.T) {
	var output bytes.Buffer
	RunStringTo(nil, `printf "zulu\nalpha\nbravo\n" | sort`, &output)

	if got, want := output.String(), "alpha\nbravo\nzulu\n"; got != want {
		t.Fatalf("pipeline output: got %q, want %q", got, want)
	}
}

func TestRunStringRedirection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.txt")

	var output bytes.Buffer
	RunStringTo(nil, `printf "first\n" > `+path, &output)
	RunStringTo(nil, `printf "second\n" >> `+path, &output)

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
	RunStringTo(nil, `cat < `+inputPath+` | grep alpha`, &output)
	if !strings.EqualFold(output.String(), "alpha\n") {
		t.Fatalf("input pipeline output: got %q, want %q", output.String(), "alpha\n")
	}
}
