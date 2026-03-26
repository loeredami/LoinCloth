package main

import (
	"fmt"
	"strconv"

	"github.com/loeredami/ungo"
)

type Scope struct {
	name      string
	overrides *ungo.SmallMap[string, string]
}

type Workspace struct {
	path   string
	scopes *ungo.LinkedList[*Scope]
	name   string
}

type State struct {
	cur_workspace int
	workspaces    *ungo.LinkedList[*Workspace]
	history       []string
	historyIndex  int

	autoCompleteMatches []string
	autoCompleteIndex   int
	lastWasTab          bool
	lastAddedLen        int

	ghostSuggestion string
	lastRowCount    int

	config Configuration
}

func (ws *Workspace) Encode() []byte {
	data := make([]byte, 0)
	ws.scopes.ForEach(func(idx int, s *Scope) {
		data = append(data, s.Encode()...)
	})
	return data
}

func (scope *Scope) Encode() []byte {
	data := make([]byte, 0)

	data = append(data, []byte(fmt.Sprintf("!new s %s\n", scope.name))...)

	scope.overrides.ForEach(func(key, value string) {
		data = append(data, []byte(fmt.Sprintf("!set %s %s\n", strconv.Quote(key), strconv.Quote(value)))...)
	})

	return data
}

func (s *State) ResetConfig() {
	s.config = DefaultConfiguration()
	ReadConfiguration(s)
}

func (ws *Workspace) Clone() *Workspace {
	nws := &Workspace{
		name:   ws.name,
		path:   ws.path,
		scopes: ungo.NewLinkedList[*Scope](),
	}

	ws.scopes.ForEach(func(idx int, s *Scope) {
		nws.scopes.Add(s.Clone())
	})

	return nws
}

func (s *Scope) Clone() *Scope {
	scope := &Scope{
		name:      s.name,
		overrides: ungo.NewSmallMap[string, string](256),
	}

	s.overrides.ForEach(func(key, val string) {
		scope.overrides.Set(key, val)
	})

	return scope
}
