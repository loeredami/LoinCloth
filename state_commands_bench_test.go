package main

import (
	"strconv"
	"testing"

	"github.com/loeredami/ungo"
)

var benchmarkStateCommand StateCmd
var benchmarkStateCommandFound bool
var benchmarkStateCommandSize int

func commandRegistryEntries() map[string]StateCmd {
	entries := make(map[string]StateCmd, StateCommands.Size())
	StateCommands.ForEach(func(name string, command StateCmd) {
		entries[name] = command
	})
	return entries
}

func TestSmallMapCollisionGrowthAndDeletion(t *testing.T) {
	type key struct {
		value int
	}
	values := ungo.NewSmallMap[key, string](1)
	for i := 0; i < 100; i++ {
		values.Set(key{value: i}, strconv.Itoa(i))
	}
	values.Set(key{value: 40}, "updated")
	values.Delete(key{value: 60})

	if got, want := values.Size(), 99; got != want {
		t.Fatalf("map size = %d, want %d", got, want)
	}
	for i := 0; i < 100; i++ {
		value, found := values.Get(key{value: i})
		switch {
		case i == 60 && found:
			t.Fatalf("deleted key %d is still present", i)
		case i == 60:
			continue
		case !found:
			t.Fatalf("key %d disappeared after collision/growth operations", i)
		case i == 40 && value != "updated":
			t.Fatalf("updated key value = %q, want updated", value)
		case i != 40 && value != strconv.Itoa(i):
			t.Fatalf("key %d value = %q, want %q", i, value, strconv.Itoa(i))
		}
	}
}

func BenchmarkCommandRegistryLookup(b *testing.B) {
	entries := commandRegistryEntries()
	const hit = "!trust-list"
	const miss = "!benchmark-command-not-registered"
	if _, exists := entries[hit]; !exists {
		b.Fatalf("benchmark hit command %q is not registered", hit)
	}

	for _, test := range []struct {
		name string
		key  string
	}{
		{name: "hit", key: hit},
		{name: "miss", key: miss},
	} {
		b.Run("SmallMap/"+test.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkStateCommand, benchmarkStateCommandFound = StateCommands.Get(test.key)
			}
		})
		b.Run("GoMap/"+test.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				benchmarkStateCommand, benchmarkStateCommandFound = entries[test.key]
			}
		})
	}
}

func BenchmarkCommandRegistryBuild(b *testing.B) {
	entries := commandRegistryEntries()
	b.Run("SmallMap/capacity-256", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			registry := ungo.NewSmallMap[string, StateCmd](256)
			for name, command := range entries {
				registry.Set(name, command)
			}
			benchmarkStateCommandSize = registry.Size()
		}
	})
	b.Run("SmallMap/capacity-64", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			registry := ungo.NewSmallMap[string, StateCmd](64)
			for name, command := range entries {
				registry.Set(name, command)
			}
			benchmarkStateCommandSize = registry.Size()
		}
	})
	b.Run("GoMap", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			registry := make(map[string]StateCmd, len(entries))
			for name, command := range entries {
				registry[name] = command
			}
			benchmarkStateCommandSize = len(registry)
		}
	})
}
