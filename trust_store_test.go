package main

import "testing"

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
