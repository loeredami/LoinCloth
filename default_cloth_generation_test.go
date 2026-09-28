package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateDefaultClothIfMissingGeneratesPlatformCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "default.cloth")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := createDefaultClothIfMissing(path); err != nil {
		t.Fatalf("create default.cloth: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	want := generatedDefaultClothCommands()
	if len(got) != len(want) {
		t.Fatalf("generated %d trust rules, want %d: %q", len(got), len(want), data)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("generated rule %d = %q, want %q", i, got[i], want[i])
		}
	}
	if len(got) == 0 {
		t.Fatal("no default trust commands were generated")
	}

	state := newInputTestState()
	var output bytes.Buffer
	for _, command := range got {
		RunStringToSource(state, command, &output, SourceDefaultCloth)
	}
	entries := state.trustStore.Entries()
	if len(entries) != len(want) {
		t.Fatalf("loaded %d default trust entries, want %d; output %q", len(entries), len(want), output.String())
	}
	for _, entry := range entries {
		if entry.Source != SourcePlatformBootstrap {
			t.Errorf("trust entry %q source = %s, want platform-bootstrap", entry.Rule, entry.Source)
		}
	}
}

func TestCreateDefaultClothIfMissingPreservesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "default.cloth")
	existing := "!local prompt custom>\n"
	if err := os.WriteFile(path, []byte(existing), 0600); err != nil {
		t.Fatal(err)
	}
	if err := createDefaultClothIfMissing(path); err != nil {
		t.Fatalf("existing default.cloth should be preserved: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != existing {
		t.Fatalf("existing default.cloth changed: %q", data)
	}
}
