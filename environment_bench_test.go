package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func commandEnvironmentWithLegacyMap(state *State) []string {
	envMap := make(map[string]string)
	for _, entry := range os.Environ() {
		pair := strings.SplitN(entry, "=", 2)
		if len(pair) == 2 {
			envMap[pair[0]] = pair[1]
		}
	}
	if state != nil {
		state.workspaces.Get(state.cur_workspace).IfPresent(func(workspace *Workspace) {
			workspace.scopes.ForEach(func(_ int, scope *Scope) {
				scope.overrides.ForEach(func(key, value string) {
					envMap[key] = value
				})
			})
		})
	}
	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", key, value))
	}
	return result
}

func commandEnvironmentWithBuiltInMap(state *State) []string {
	environment := os.Environ()
	envMap := make(map[string]string, len(environment))
	for _, entry := range environment {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			envMap[key] = value
		}
	}
	if state != nil {
		state.workspaces.Get(state.cur_workspace).IfPresent(func(workspace *Workspace) {
			workspace.scopes.ForEach(func(_ int, scope *Scope) {
				scope.overrides.ForEach(func(key, value string) {
					envMap[key] = value
				})
			})
		})
	}
	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, key+"="+value)
	}
	return result
}

func TestCommandEnvironmentSmallMapMatchesBuiltInMap(t *testing.T) {
	t.Setenv("LOINCLOTH_TEST_COMMAND_ENV", "host")
	state := stateWithScopes(2)
	state.workspaces.Get(0).Value().scopes.Get(0).Value().overrides.Set("LOINCLOTH_TEST_COMMAND_ENV", "outer")
	state.workspaces.Get(0).Value().scopes.Get(1).Value().overrides.Set("LOINCLOTH_TEST_COMMAND_ENV", "inner")
	state.workspaces.Get(0).Value().scopes.Get(1).Value().overrides.Set("LOINCLOTH_TEST_COMMAND_ENV_ADDED", "scope-only")

	asMap := func(entries []string) map[string]string {
		result := make(map[string]string, len(entries))
		for _, entry := range entries {
			key, value, ok := strings.Cut(entry, "=")
			if !ok {
				t.Fatalf("malformed environment entry %q", entry)
			}
			result[key] = value
		}
		return result
	}
	got, want := asMap(commandEnvironment(state)), asMap(commandEnvironmentWithBuiltInMap(state))
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s: SmallMap value = %q, built-in map value = %q", key, got[key], value)
		}
	}
	for key, value := range got {
		if want[key] != value {
			t.Errorf("%s: unexpected SmallMap value %q (built-in map has %q)", key, value, want[key])
		}
	}
}

var benchmarkCommandEnvironment []string

func benchmarkCommandEnvironmentState() *State {
	state := stateWithScopes(3)
	state.workspaces.Get(0).Value().scopes.Get(0).Value().overrides.Set("PATH", "/workspace/bin")
	state.workspaces.Get(0).Value().scopes.Get(1).Value().overrides.Set("HOME", "/workspace/home")
	state.workspaces.Get(0).Value().scopes.Get(2).Value().overrides.Set("LOINCLOTH_BUILD", "release")
	return state
}

func BenchmarkCommandEnvironmentLegacyMap(b *testing.B) {
	state := benchmarkCommandEnvironmentState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkCommandEnvironment = commandEnvironmentWithLegacyMap(state)
	}
}

func BenchmarkCommandEnvironmentBuiltInMap(b *testing.B) {
	state := benchmarkCommandEnvironmentState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkCommandEnvironment = commandEnvironmentWithBuiltInMap(state)
	}
}

func BenchmarkCommandEnvironmentSmallMap(b *testing.B) {
	state := benchmarkCommandEnvironmentState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkCommandEnvironment = commandEnvironment(state)
	}
}
