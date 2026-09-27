package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loeredami/ungo"
)

func newInputTestState() *State {
	return &State{
		workspaces: ungo.NewLinkedList[*Workspace](),
		config:     DefaultConfiguration(),
	}
}

func TestReadRawInputPreservesPastedCommandsAndCRLF(t *testing.T) {
	state := newInputTestState()
	input := strings.NewReader("first\r\nsecond\r\n")

	if got := readRawInputFrom(state, "> ", input); got != "first" {
		t.Fatalf("first pasted command: got %q, want %q", got, "first")
	}
	if got := readRawInputFrom(state, "> ", input); got != "second" {
		t.Fatalf("second pasted command: got %q, want %q", got, "second")
	}
	if len(state.history) != 2 || state.history[0] != "first" || state.history[1] != "second" {
		t.Fatalf("pasted command history: %#v", state.history)
	}
}

func TestReadRawInputPreservesLargePastedBatch(t *testing.T) {
	state := newInputTestState()
	const commandCount = 700
	input := strings.NewReader(strings.Repeat("x\r\n", commandCount))
	for i := 0; i < commandCount; i++ {
		if got := readRawInputFrom(state, "> ", input); got != "x" {
			t.Fatalf("pasted command %d: got %q, want %q", i, got, "x")
		}
	}
}

func TestReadRawInputContinuationAndCancellation(t *testing.T) {
	t.Run("CRLF continuation joins physical lines", func(t *testing.T) {
		state := newInputTestState()
		input := strings.NewReader("echo first\\\r\nsecond\r\n")
		if got, want := readRawInputFrom(state, "> ", input), "echo first\nsecond"; got != want {
			t.Fatalf("continued command: got %q, want %q", got, want)
		}
	})

	t.Run("Ctrl+C cancels the accumulated command", func(t *testing.T) {
		state := newInputTestState()
		input := strings.NewReader("echo first\\\n\x03next\n")
		if got := readRawInputFrom(state, "> ", input); got != "" {
			t.Fatalf("cancelled command: got %q, want empty", got)
		}
		if len(state.history) != 0 {
			t.Fatalf("cancelled command entered history: %#v", state.history)
		}

		if got := readRawInputFrom(state, "> ", input); got != "next" {
			t.Fatalf("command after cancellation: got %q, want %q", got, "next")
		}
	})
}

func TestRunNonInteractivePastedBatchAndContinuation(t *testing.T) {
	state := newInputTestState()
	var got [][]string
	RegisterCmd("!input-batch-probe", func(state *State, command []string) ungo.Optional[error] {
		got = append(got, append([]string(nil), command...))
		if state.commandSource != SourceNonInteractive {
			return ungo.Some(fmt.Errorf("command source = %s, want %s", state.commandSource, SourceNonInteractive))
		}
		return ungo.None[error]()
	})

	input := strings.NewReader("!input-batch-probe first\r\n!input-batch-probe second\\\r\nthird\r\nexit\r\n!input-batch-probe ignored\r\n")
	if err := runNonInteractive(state, input); err != nil {
		t.Fatalf("runNonInteractive returned error: %v", err)
	}
	want := [][]string{
		{"!input-batch-probe", "first"},
		{"!input-batch-probe", "second", "third"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("commands run: got %#v, want %#v", got, want)
	}
}

func TestRunStringPreservesNestedClothSource(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "inner.cloth")
	outer := filepath.Join(dir, "outer.cloth")
	if err := os.WriteFile(inner, []byte("!source-context-probe\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outer, []byte(`!wear "`+inner+`"`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	var sources []CommandSource
	RegisterCmd("!source-context-probe", func(state *State, command []string) ungo.Optional[error] {
		sources = append(sources, state.commandSource)
		return ungo.None[error]()
	})

	state := newInputTestState()
	var output bytes.Buffer
	RunStringToSource(state, "!source-context-probe", &output, SourceInteractive)
	RunStringToSource(state, "!source-context-probe", &output, SourceDefaultCloth)
	RunStringToSource(state, `!wear "`+outer+`"`, &output, SourceDefaultCloth)
	want := []CommandSource{SourceInteractive, SourceDefaultCloth, SourceClothFile}
	if fmt.Sprint(sources) != fmt.Sprint(want) {
		t.Fatalf("command sources: got %#v, want %#v", sources, want)
	}
	if state.commandSource != SourceInteractive {
		t.Fatalf("command source not restored after nested load: got %s", state.commandSource)
	}
}
