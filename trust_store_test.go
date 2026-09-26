package main

import (
	"os"
	"testing"
)

func TestTrustStoreMatchesExactPath(t *testing.T) {
	kind, rule := ParseTrustRule("/usr/bin/example")
	if kind != TrustExactPath {
		t.Fatalf("rule kind: got %v, want exact path", kind)
	}

	var store TrustStore
	store.Add(TrustEntry{Rule: rule, Kind: kind, Source: SourceInteractive})
	if !store.Allows("/usr/bin/example") {
		t.Fatal("exact path was not allowed")
	}
	if store.Allows("/usr/local/bin/example") {
		t.Fatal("different path was allowed by exact rule")
	}
}

func TestTrustStoreMatchesBasename(t *testing.T) {
	kind, rule := ParseTrustRule("explorer.exe")
	if kind != TrustBasename {
		t.Fatalf("rule kind: got %v, want basename", kind)
	}

	var store TrustStore
	store.Add(TrustEntry{Rule: rule, Kind: kind, Source: SourceInteractive})
	if !store.Allows(`C:\Windows\explorer.exe`) {
		t.Fatal("basename rule did not match Windows path")
	}
}

func TestTrustStoreMatchesExplicitGlob(t *testing.T) {
	kind, rule := ParseTrustRule("python*")
	if kind != TrustGlob {
		t.Fatalf("rule kind: got %v, want glob", kind)
	}

	var store TrustStore
	store.Add(TrustEntry{Rule: rule, Kind: kind, Source: SourceInteractive})
	if !store.Allows("/usr/bin/python3") {
		t.Fatal("glob rule did not match executable basename")
	}
	if store.Allows("/usr/bin/ruby") {
		t.Fatal("glob rule matched unrelated executable")
	}
}

func TestTrustStoreDeduplicatesEntries(t *testing.T) {
	var store TrustStore
	entry := TrustEntry{Rule: "example", Kind: TrustBasename, Source: SourceInteractive}
	store.Add(entry)
	store.Add(entry)
	if len(store.Entries()) != 1 {
		t.Fatalf("entry count: got %d, want 1", len(store.Entries()))
	}
}

func TestTrustStorePersistsAndLoads(t *testing.T) {
	path := t.TempDir() + "/trust.json"
	original := TrustStore{}
	original.Add(TrustEntry{Rule: "/usr/bin/example", Kind: TrustExactPath, Source: SourceInteractive})
	original.Add(TrustEntry{Rule: "python*", Kind: TrustGlob, Source: SourceDefaultCloth})

	if err := SaveTrustStore(path, original); err != nil {
		t.Fatalf("save trust store: %v", err)
	}
	loaded, err := LoadTrustStore(path)
	if err != nil {
		t.Fatalf("load trust store: %v", err)
	}
	if len(loaded.Entries()) != 2 || !loaded.Allows("/usr/bin/example") || !loaded.Allows("/usr/bin/python3") {
		t.Fatalf("loaded trust entries did not match: %#v", loaded.Entries())
	}
}

func TestTrustStoreRejectsInvalidData(t *testing.T) {
	path := t.TempDir() + "/trust.json"
	if err := os.WriteFile(path, []byte(`{"version":99,"entries":[]}`), 0600); err != nil {
		t.Fatalf("write invalid trust store: %v", err)
	}
	if _, err := LoadTrustStore(path); err == nil {
		t.Fatal("invalid trust store was accepted")
	}
}

func TestTrustStoreRemove(t *testing.T) {
	var store TrustStore
	entry := TrustEntry{Rule: "example", Kind: TrustBasename, Source: SourceInteractive}
	store.Add(entry)
	if !store.Remove(entry) || store.Remove(entry) || len(store.Entries()) != 0 {
		t.Fatalf("trust entry was not removed correctly")
	}
}
