package main

import (
	"path/filepath"
	"testing"

	"github.com/loeredami/ungo"
)

func stateWithScopes(scopeCount int) *State {
	state := &State{workspaces: ungo.NewLinkedList[*Workspace]()}
	workspace := &Workspace{
		scopes: ungo.NewLinkedList[*Scope](),
	}
	for i := 0; i < scopeCount; i++ {
		workspace.scopes.Add(&Scope{
			overrides: ungo.NewSmallMap[string, string](8),
		})
	}
	state.workspaces.Add(workspace)
	return state
}

func TestGetEnvValueScopePrecedenceAndEnvironmentFallback(t *testing.T) {
	t.Setenv("LOINCLOTH_TEST_ENV_LOOKUP", "system")
	state := stateWithScopes(3)
	workspace := state.workspaces.Get(0).Value()
	workspace.scopes.Get(0).Value().overrides.Set("LOINCLOTH_TEST_ENV_LOOKUP", "old scope")
	workspace.scopes.Get(2).Value().overrides.Set("LOINCLOTH_TEST_ENV_LOOKUP", "new scope")

	value := GetEnvValue(state, "LOINCLOTH_TEST_ENV_LOOKUP")
	if !value.HasValue() {
		t.Fatal("scope override was not found")
	}
	if got, want := value.Value(), []string{"new", "scope"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("newest scope value: got %#v, want %#v", got, want)
	}

	workspace.scopes.Get(2).Value().overrides.Delete("LOINCLOTH_TEST_ENV_LOOKUP")
	workspace.scopes.Get(0).Value().overrides.Delete("LOINCLOTH_TEST_ENV_LOOKUP")
	value = GetEnvValue(state, "LOINCLOTH_TEST_ENV_LOOKUP")
	if !value.HasValue() {
		t.Fatal("environment fallback was not found")
	}
	if got, want := value.Value(), []string{"system"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("environment value: got %#v, want %#v", got, want)
	}
}

func TestGetEnvValueDistinguishesUnsetAndEmptyEnvironment(t *testing.T) {
	const key = "LOINCLOTH_TEST_EMPTY_ENV_LOOKUP"
	t.Setenv(key, "")
	state := stateWithScopes(0)

	value := GetEnvValue(state, key)
	if !value.HasValue() {
		t.Fatal("empty but set environment variable was treated as unset")
	}
	if got := value.Value(); len(got) != 1 || got[0] != "" {
		t.Fatalf("empty environment value: got %#v, want [\"\"]", got)
	}

	value = GetEnvValue(state, key+"_UNSET")
	if value.HasValue() {
		t.Fatalf("unset environment variable unexpectedly found: %#v", value.Value())
	}
}

func TestGetEnvValueHandlesMissingWorkspaceState(t *testing.T) {
	states := []*State{
		nil,
		{},
		{workspaces: ungo.NewLinkedList[*Workspace]()},
		{workspaces: ungo.ListOf[*Workspace](nil)},
		{workspaces: ungo.ListOf(&Workspace{})},
		{
			workspaces:    ungo.ListOf(&Workspace{scopes: ungo.NewLinkedList[*Scope]()}),
			cur_workspace: 1,
		},
	}
	for i, state := range states {
		if value := GetEnvValue(state, "PATH"); value.HasValue() {
			t.Errorf("state %d unexpectedly returned environment value %#v", i, value.Value())
		}
	}
}

func TestWorkspaceCommandsRejectMissingCurrentWorkspace(t *testing.T) {
	commands := [][]string{
		{"!new", "w"},
		{"!new", "s", "scope"},
		{"!label", "workspace"},
		{"!drop", "scope"},
		{"!set", "KEY", "value"},
		{"!set-ifn", "KEY", "value"},
		{"!snapshot-ws", filepath.Join(t.TempDir(), "snapshot.cloth")},
	}
	states := []struct {
		name  string
		state *State
	}{
		{name: "no workspaces", state: &State{workspaces: ungo.NewLinkedList[*Workspace]()}},
		{name: "invalid current index", state: &State{
			workspaces:    ungo.ListOf(&Workspace{scopes: ungo.NewLinkedList[*Scope]()}),
			cur_workspace: 1,
		}},
	}

	for _, test := range states {
		t.Run(test.name, func(t *testing.T) {
			for _, command := range commands {
				result := HandleStateCommands(test.state, command)
				if !result.HasValue() {
					t.Errorf("%v unexpectedly succeeded without a current workspace", command)
				}
			}
		})
	}
}

func TestWorkspaceCommandsRejectWorkspaceWithoutScopeList(t *testing.T) {
	state := &State{
		workspaces: ungo.ListOf(&Workspace{}),
	}
	for _, command := range [][]string{
		{"!new", "s", "scope"},
		{"!set", "KEY", "value"},
		{"!snapshot-ws", filepath.Join(t.TempDir(), "snapshot.cloth")},
	} {
		result := HandleStateCommands(state, command)
		if !result.HasValue() {
			t.Errorf("%v unexpectedly succeeded with a malformed workspace", command)
		}
	}
}

func TestWorkspaceAndScopeCommandsUseValidCurrentWorkspace(t *testing.T) {
	state := stateWithScopes(1)
	initialCount := state.workspaces.Size()

	if result := HandleStateCommands(state, []string{"!new", "w"}); result.HasValue() {
		t.Fatalf("creating workspace returned error: %v", result.Value())
	}
	if got := state.workspaces.Size(); got != initialCount+1 {
		t.Fatalf("workspace count = %d, want %d", got, initialCount+1)
	}
	if got := state.cur_workspace; got != initialCount {
		t.Fatalf("current workspace = %d after creation, want new workspace index %d", got, initialCount)
	}
	current, err := currentWorkspace(state)
	if err != nil {
		t.Fatalf("get newly created current workspace: %v", err)
	}
	if current.path != "" || current.scopes == nil || current.scopes.Size() != 0 {
		t.Fatalf("new workspace was not initialized correctly: %#v", current)
	}

	if result := HandleStateCommands(state, []string{"!new", "s", "test-scope"}); result.HasValue() {
		t.Fatalf("creating scope returned error: %v", result.Value())
	}
	if result := HandleStateCommands(state, []string{"!set", "LOIN_TEST_SCOPE_VALUE", "session-value"}); result.HasValue() {
		t.Fatalf("setting scope variable returned error: %v", result.Value())
	}
	value := GetEnvValue(state, "LOIN_TEST_SCOPE_VALUE")
	if !value.HasValue() || len(value.Value()) != 1 || value.Value()[0] != "session-value" {
		t.Fatalf("scope variable = %v, want [session-value]", value)
	}
}

func TestNewWorkspaceRejectsUnknownKindWithoutChangingState(t *testing.T) {
	state := stateWithScopes(1)
	count := state.workspaces.Size()

	result := HandleStateCommands(state, []string{"!new", "workspace.go"})
	if !result.HasValue() {
		t.Fatal("unknown workspace kind unexpectedly succeeded")
	}
	if got := state.workspaces.Size(); got != count {
		t.Fatalf("invalid !new changed workspace count to %d, want %d", got, count)
	}
}

var benchmarkEnvValue ungo.Optional[[]string]

func BenchmarkGetEnvValueManyScopes(b *testing.B) {
	b.Setenv("LOINCLOTH_BENCH_ENV_LOOKUP", "present")
	state := stateWithScopes(64)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkEnvValue = GetEnvValue(state, "LOINCLOTH_BENCH_ENV_LOOKUP")
	}
}
