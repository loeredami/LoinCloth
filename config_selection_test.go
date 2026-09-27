package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loeredami/ungo"
)

func TestValidateSelectedCloth(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		err := validateSelectedCloth(filepath.Join(t.TempDir(), "missing.cloth"))
		if err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("missing file error: %v", err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		dir := t.TempDir()
		err := validateSelectedCloth(dir)
		if err == nil || !strings.Contains(err.Error(), "directory") {
			t.Fatalf("directory error: %v", err)
		}
	})

	t.Run("regular file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "development.cloth")
		if err := os.WriteFile(path, []byte(""), 0600); err != nil {
			t.Fatal(err)
		}
		if err := validateSelectedCloth(path); err != nil {
			t.Fatalf("regular file rejected: %v", err)
		}
	})
}

func TestSelectedClothWarning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "development.cloth")
	if warning := selectedClothWarning(path); !strings.Contains(warning, "outside") {
		t.Fatalf("expected outside-location warning, got %q", warning)
	}
}

func TestWriteSecurityStatus(t *testing.T) {
	state := &State{
		configPath:   "development.cloth",
		configSource: SourceDevelopmentCloth,
		workspaces:   ungo.NewLinkedList[*Workspace](),
	}
	var output strings.Builder
	if err := writeSecurityStatus(state, &output); err != nil {
		t.Fatal(err)
	}
	result := output.String()
	for _, expected := range []string{
		"Configuration path: development.cloth",
		"Configuration source: development .cloth",
		"Configuration trusted: No",
		"Executable trust store: unavailable",
		"Privilege state: normal",
	} {
		if !strings.Contains(result, expected) {
			t.Fatalf("status missing %q in %q", expected, result)
		}
	}
}
