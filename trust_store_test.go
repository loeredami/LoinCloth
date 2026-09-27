package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
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

func TestTrustStorePersistsAndLoads(t *testing.T) {
	path := trustStoreTestPath(t)
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

func TestTrustStoreConcurrentReadAndWriteStaysValid(t *testing.T) {
	path := trustStoreTestPath(t)
	const writers = 8
	done := make(chan error, writers)
	for i := 0; i < writers; i++ {
		go func(index int) {
			store := TrustStore{}
			store.Add(TrustEntry{
				Rule:   fmt.Sprintf("/usr/bin/tool-%d", index),
				Kind:   TrustExactPath,
				Source: SourceInteractive,
			})
			done <- SaveTrustStore(path, store)
		}(i)
	}
	for i := 0; i < writers; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent trust store save: %v", err)
		}
	}
	loaded, err := LoadTrustStore(path)
	if err != nil {
		t.Fatalf("load trust store after concurrent saves: %v", err)
	}
	if len(loaded.Entries()) != 1 {
		t.Fatalf("concurrent writes left invalid entry count: %#v", loaded.Entries())
	}
}

func TestConcurrentTrustStoreTransactionsRejectStaleWriter(t *testing.T) {
	path := trustStoreTestPath(t)
	initial := TrustStore{}
	initial.Add(TrustEntry{Rule: "/usr/bin/initial", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, initial); err != nil {
		t.Fatalf("save initial trust store: %v", err)
	}
	_, expectedHash, expectedExists, err := loadTrustStoreSnapshot(path)
	if err != nil {
		t.Fatalf("read initial trust snapshot: %v", err)
	}

	type result struct {
		rule string
		err  error
	}
	results := make(chan result, 2)
	for _, rule := range []string{"/usr/bin/first", "/usr/bin/second"} {
		go func(rule string) {
			candidate := TrustStore{}
			candidate.Add(TrustEntry{Rule: rule, Kind: TrustExactPath, Source: SourceInteractive})
			_, _, err := SaveTrustStoreIfUnchanged(path, expectedHash, expectedExists, candidate)
			results <- result{rule: rule, err: err}
		}(rule)
	}

	successes := 0
	var winningRule string
	for range 2 {
		result := <-results
		if result.err == nil {
			successes++
			winningRule = result.rule
		} else if !strings.Contains(result.err.Error(), "changed since it was loaded") {
			t.Fatalf("unexpected stale transaction error: %v", result.err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful transactions = %d, want exactly 1", successes)
	}
	loaded, err := LoadTrustStore(path)
	if err != nil {
		t.Fatalf("load trust store after transactions: %v", err)
	}
	if len(loaded.Entries()) != 1 || loaded.Entries()[0].Rule != winningRule {
		t.Fatalf("stale transaction overwrote committed rule: %#v, winner %q", loaded.Entries(), winningRule)
	}
}

func TestTrustStoreLoadRejectsWorkspaceCommandRule(t *testing.T) {
	path := trustStoreTestPath(t)
	data := []byte(`{"version":1,"entries":[{"Rule":"!wear","Kind":1,"Source":0}]}`)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write invalid trust store: %v", err)
	}
	if _, err := LoadTrustStore(path); err == nil || !strings.Contains(err.Error(), "workspace commands") {
		t.Fatalf("workspace trust rule error: got %v", err)
	}
}

func TestTrustStoreSaveRejectsWorkspaceCommandRule(t *testing.T) {
	path := trustStoreTestPath(t)
	store := TrustStore{entries: []TrustEntry{
		{Rule: "!wear", Kind: TrustBasename, Source: SourceInteractive},
	}}
	if err := SaveTrustStore(path, store); err == nil || !strings.Contains(err.Error(), "workspace commands") {
		t.Fatalf("workspace trust rule save error: got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid trust store was written, stat error: %v", err)
	}
}

func TestTrustStoreRejectsInvalidData(t *testing.T) {
	path := trustStoreTestPath(t)
	if err := os.WriteFile(path, []byte(`{"version":99,"entries":[]}`), 0600); err != nil {
		t.Fatalf("write invalid trust store: %v", err)
	}
	if _, err := LoadTrustStore(path); err == nil {
		t.Fatal("invalid trust store was accepted")
	}
}

func TestCorruptTrustStoreFailsClosedAndManagementDoesNotOverwrite(t *testing.T) {
	path := trustStoreTestPath(t)
	corrupt := []byte(`{"version":99,"entries":[]}`)
	if err := os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatalf("write corrupt store: %v", err)
	}
	state := &State{
		commandSource:    SourceInteractive,
		interactiveInput: true,
		trustStore:       TrustStore{},
	}
	kind, rule := ParseTrustRule("/usr/bin/example")
	state.trustStore.Add(TrustEntry{Rule: rule, Kind: kind, Source: SourceInteractive})
	if err := loadTrustStoreIntoState(state, path); err == nil {
		t.Fatal("corrupt trust store loaded without error")
	}
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("invalid trust entries remained active: %#v", state.trustStore.Entries())
	}
	if state.trustStoreError == nil {
		t.Fatal("trust store failure was not recorded")
	}

	for _, command := range [][]string{
		{"!trust", "/usr/bin/new-command"},
		{"!trust-list"},
		{"!untrust", "/usr/bin/example"},
	} {
		result := HandleStateCommands(state, command)
		if !result.HasValue() || !strings.Contains(result.Value().Error(), "trust store unavailable") {
			t.Errorf("%q was not blocked on rejected store: %v", command, result)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original corrupt store: %v", err)
	}
	if !bytes.Equal(data, corrupt) {
		t.Fatalf("trust management modified corrupt data: got %q, want %q", data, corrupt)
	}
}

func TestTrustStoreChangesAfterLoadFailClosed(t *testing.T) {
	path := trustStoreTestPath(t)
	original := TrustStore{}
	original.Add(TrustEntry{Rule: "/usr/bin/original", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, original); err != nil {
		t.Fatalf("save initial store: %v", err)
	}

	state := &State{}
	if err := loadTrustStoreIntoState(state, path); err != nil {
		t.Fatalf("load trust store: %v", err)
	}
	if !state.trustStore.Allows("/usr/bin/original") {
		t.Fatal("initial trust entry was not loaded")
	}

	changed := TrustStore{}
	changed.Add(TrustEntry{Rule: "/usr/bin/replacement", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, changed); err != nil {
		t.Fatalf("replace trust store externally: %v", err)
	}

	if err := trustStoreAvailable(state); err == nil || !strings.Contains(err.Error(), "changed after loading") {
		t.Fatalf("changed store availability error: got %v", err)
	}
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("stale trust entries remained active: %#v", state.trustStore.Entries())
	}
	if state.trustStore.Allows("/usr/bin/original") {
		t.Fatal("stale trust entry remained authorized")
	}
}

func TestChangedTrustStoreCannotAuthorizeCommand(t *testing.T) {
	path := trustStoreTestPath(t)
	store := TrustStore{}
	store.Add(TrustEntry{Rule: "/usr/bin/example", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, store); err != nil {
		t.Fatalf("save trust store: %v", err)
	}
	state := &State{commandSource: SourceNonInteractive}
	if err := loadTrustStoreIntoState(state, path); err != nil {
		t.Fatalf("load trust store: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"entries":[]}`), 0600); err != nil {
		t.Fatalf("modify trust store externally: %v", err)
	}
	if authorizeExecutable(state, "/usr/bin/example", nil) {
		t.Fatal("command remained authorized by stale trust entry")
	}
}

func TestTrustStoreDeletionAfterLoadFailsClosed(t *testing.T) {
	path := trustStoreTestPath(t)
	store := TrustStore{}
	store.Add(TrustEntry{Rule: "/usr/bin/example", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, store); err != nil {
		t.Fatalf("save trust store: %v", err)
	}
	state := &State{}
	if err := loadTrustStoreIntoState(state, path); err != nil {
		t.Fatalf("load trust store: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("delete trust store externally: %v", err)
	}
	if err := trustStoreAvailable(state); err == nil || !strings.Contains(err.Error(), "changed after loading") {
		t.Fatalf("deleted store availability error: got %v", err)
	}
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("deleted store trust remained active: %#v", state.trustStore.Entries())
	}
}

func TestTrustStoreRecoveryRequiresConfirmationAndReloadsValidFile(t *testing.T) {
	path := trustStoreTestPath(t)
	initial := TrustStore{}
	initial.Add(TrustEntry{Rule: "/usr/bin/original", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, initial); err != nil {
		t.Fatalf("save initial store: %v", err)
	}
	state := &State{
		commandSource:    SourceInteractive,
		interactiveInput: true,
	}
	if err := loadTrustStoreIntoState(state, path); err != nil {
		t.Fatalf("load initial store: %v", err)
	}

	repaired := TrustStore{}
	repaired.Add(TrustEntry{Rule: "/usr/bin/repaired", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, repaired); err != nil {
		t.Fatalf("write reviewed store: %v", err)
	}
	if err := trustStoreAvailable(state); err == nil {
		t.Fatal("modified store was not detected")
	}

	withPromptInput(t, "n")
	declined := HandleStateCommands(state, []string{"!trust-reload"})
	if declined.HasValue() {
		t.Fatalf("declined reload returned error: %v", declined.Value())
	}
	if state.trustStoreError == nil || len(state.trustStore.Entries()) != 0 {
		t.Fatal("declining reload restored trust")
	}

	withPromptInput(t, "y")
	reloaded := HandleStateCommands(state, []string{"!trust-reload"})
	if reloaded.HasValue() {
		t.Fatalf("confirmed reload failed: %v", reloaded.Value())
	}
	if state.trustStoreError != nil {
		t.Fatalf("trust store remains unavailable after reload: %v", state.trustStoreError)
	}
	if !state.trustStore.Allows("/usr/bin/repaired") || state.trustStore.Allows("/usr/bin/original") {
		t.Fatalf("reloaded trust entries are incorrect: %#v", state.trustStore.Entries())
	}
}

func TestTrustStoreRecoveryKeepsInvalidFileFailClosed(t *testing.T) {
	path := trustStoreTestPath(t)
	initial := TrustStore{}
	initial.Add(TrustEntry{Rule: "/usr/bin/original", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, initial); err != nil {
		t.Fatalf("save initial store: %v", err)
	}
	state := &State{
		commandSource:    SourceInteractive,
		interactiveInput: true,
	}
	if err := loadTrustStoreIntoState(state, path); err != nil {
		t.Fatalf("load initial store: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"version":99,"entries":[]}`), 0600); err != nil {
		t.Fatalf("corrupt trust store: %v", err)
	}
	if err := trustStoreAvailable(state); err == nil {
		t.Fatal("corrupt store modification was not detected")
	}

	withPromptInput(t, "y")
	result := HandleStateCommands(state, []string{"!trust-reload"})
	if !result.HasValue() || !strings.Contains(result.Value().Error(), "unsupported trust store version 99") {
		t.Fatalf("invalid recovery result: %v", result)
	}
	if state.trustStoreError == nil || len(state.trustStore.Entries()) != 0 {
		t.Fatal("invalid trust store reload did not remain fail-closed")
	}
}

func TestTrustMutationDoesNotOverwriteChangedStore(t *testing.T) {
	withPromptInput(t, "y")
	path := trustStoreTestPath(t)
	original := TrustStore{}
	original.Add(TrustEntry{Rule: "/usr/bin/original", Kind: TrustExactPath, Source: SourceInteractive})
	if err := SaveTrustStore(path, original); err != nil {
		t.Fatalf("save initial store: %v", err)
	}
	state := &State{
		commandSource:    SourceInteractive,
		interactiveInput: true,
	}
	if err := loadTrustStoreIntoState(state, path); err != nil {
		t.Fatalf("load initial store: %v", err)
	}

	external := []byte(`{"version":1,"entries":[{"Rule":"/usr/bin/external","Kind":0,"Source":0}]}`)
	if err := os.WriteFile(path, external, 0600); err != nil {
		t.Fatalf("modify store externally: %v", err)
	}
	result := HandleStateCommands(state, []string{"!trust", "/usr/bin/new"})
	if !result.HasValue() || !strings.Contains(result.Value().Error(), "changed after loading") {
		t.Fatalf("trust mutation did not reject changed store: %v", result)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read changed store: %v", err)
	}
	if !bytes.Equal(data, external) {
		t.Fatalf("trust mutation overwrote external data: got %q, want %q", data, external)
	}
}

func TestRejectedTrustStoreBlocksPersistentPromptChoice(t *testing.T) {
	withPromptInput(t, "2")
	path := trustStoreTestPath(t)
	state := &State{
		trustStorePath:  path,
		trustStoreError: fmt.Errorf("unsupported trust store version"),
	}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if promptExecutableTrust(state, "/usr/bin/example", writer) {
		t.Fatal("persistent approval succeeded with unavailable trust store")
	}
	writer.Flush()
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("unavailable trust store acquired entries: %#v", state.trustStore.Entries())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("persistent approval overwrote rejected store, stat error: %v", err)
	}
}

func TestRejectedTrustStoreStillAllowsRunOnce(t *testing.T) {
	withPromptInput(t, "1")
	state := &State{trustStoreError: fmt.Errorf("invalid trust store")}
	var output bytes.Buffer
	writer := bufio.NewWriter(&output)
	if !promptExecutableTrust(state, "/usr/bin/example", writer) {
		t.Fatalf("Run Once should remain available when persistence is unavailable: %s", output.String())
	}
	writer.Flush()
	if len(state.trustStore.Entries()) != 0 {
		t.Fatalf("Run Once added trust to unavailable store: %#v", state.trustStore.Entries())
	}
}

func TestSecurityStatusReportsRejectedTrustStore(t *testing.T) {
	state := &State{
		configPath:      "development.cloth",
		trustStorePath:  "/tmp/trust.json",
		trustStoreError: fmt.Errorf("unsupported trust store version"),
	}
	var output bytes.Buffer
	if err := writeSecurityStatus(state, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Executable trust store: unavailable (unsupported trust store version)") {
		t.Fatalf("status did not expose rejected store state: %q", output.String())
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
