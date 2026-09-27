package main

import (
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

var benchmarkEnvValue ungo.Optional[[]string]

func BenchmarkGetEnvValueManyScopes(b *testing.B) {
	b.Setenv("LOINCLOTH_BENCH_ENV_LOOKUP", "present")
	state := stateWithScopes(64)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkEnvValue = GetEnvValue(state, "LOINCLOTH_BENCH_ENV_LOOKUP")
	}
}
