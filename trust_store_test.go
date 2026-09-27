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

func TestTrustStoreRejectsWorkspaceCommandRules(t *testing.T) {
	for _, rule := range []string{"!wear", " !wear", "!wear*"} {
		t.Run(rule, func(t *testing.T) {
			entry := TrustEntry{Rule: rule, Kind: TrustBasename, Source: SourceInteractive}
			var store TrustStore
			store.Add(entry)
			if len(store.Entries()) != 0 {
				t.Fatalf("workspace rule was added: %#v", store.Entries())
			}
			if entry.Matches("!wear") {
				t.Fatal("workspace rule matched an executable")
			}
			if store.Allows("!wear") {
				t.Fatal("workspace command was authorized")
			}
		})
	}
}

func TestTrustStoreRemove(t *testing.T) {
	var store TrustStore
	entry := TrustEntry{Rule: "example", Kind: TrustBasename, Source: SourceInteractive}
	store.Add(entry)
	if !store.Remove(entry) || store.Remove(entry) || len(store.Entries()) != 0 {
		t.Fatal("trust entry was not removed correctly")
	}
}

func TestTrustStoreEntriesReturnsCopy(t *testing.T) {
	var store TrustStore
	store.Add(TrustEntry{Rule: "example", Kind: TrustBasename, Source: SourceInteractive})
	entries := store.Entries()
	entries[0].Rule = "modified"
	if !store.Allows("/usr/bin/example") {
		t.Fatal("mutating returned entries changed session trust")
	}
}
