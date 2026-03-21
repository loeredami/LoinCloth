package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/loeredami/ungo"
)

func GetEnvValue(state *State, key string) ungo.Optional[[]string] {
	ws_opt := state.workspaces.Get(state.cur_workspace)

	if !ws_opt.HasValue() {
		return ungo.None[[]string]()
	}
	ws := ws_opt.Value()
	result := ungo.None[[]string]()
	var found_in_os bool = false
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			if pair[0] == key {
				result = ungo.Some([]string{})

				Lex(pair[1]).ForEach(func(i int, tk Token) {
					if found_in_os {
						return
					}
					result = ungo.Some(append(result.Value(), tk.Value.OrElse("")))
					found_in_os = true
				})
			}
		}
		if found_in_os {
			break
		}
	}
	i := ws.scopes.Size() - 1

	for {
		if i < 0 {
			return result
		}

		sc_opt := ws.scopes.Get(i)

		if sc_opt.HasValue() {
			sc := sc_opt.Value()

			if val, ok := sc.overrides.Get(key); ok {
				tokens := Lex(val)

				command_strings := []string{}
				tokens.ForEach(func(idx int, token Token) {
					if token.Type == EndOfInput {
						return
					}

					if token.Type == Path {
						token.Value.IfPresent(func(val string) {
							command_strings = append(command_strings, UnformatPathIfInHome(val))
						})
						return
					}

					token.Value.IfPresent(func(val string) {
						command_strings = append(command_strings, val)
					})
				})

				result = ungo.Some(command_strings)
				break
			}

		}

		i--
	}

	return result
}

func HandleStateCommands(state *State, command []string) ungo.Optional[error] {
	switch command[0] {
	case "!new":
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected argument 'w' for workspace or 's' for scope"))
		}

		switch command[1] {
		case "w":
			state.workspaces.Add(&Workspace{
				path:   state.workspaces.Get(state.cur_workspace).Value().path,
				scopes: ungo.NewLinkedList[*Scope](),
			})
		case "s":
			if len(command) < 3 {
				return ungo.Some(fmt.Errorf("error creating scope: no name given"))
			}
			scope := &Scope{
				name:      command[2],
				overrides: ungo.NewSmallMap[string, string](256),
			}

			state.workspaces.Get(state.cur_workspace).IfPresent(func(w *Workspace) {
				w.scopes.Add(scope)
			})
		default:
			return ungo.Some(fmt.Errorf("expected argument 'workspace'"))
		}

		return ungo.None[error]()

	case "!switch":
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected index of workspace"))
		}

		idx, err := strconv.ParseUint(command[1], 10, 64)

		if err != nil {
			return ungo.Some(fmt.Errorf("could not parse index: %v", err))
		}

		if idx >= uint64(state.workspaces.Size()) {
			return ungo.Some(fmt.Errorf("workspace at %d does not exist.", idx))
		}

		prev := state.cur_workspace
		state.cur_workspace = int(idx)

		err = os.Chdir(state.workspaces.Get(state.cur_workspace).Value().path)

		if err != nil {
			state.cur_workspace = prev
			state.workspaces.Remove(int(idx))
			return ungo.Some(fmt.Errorf("could not open workspace at %d: %v", idx, err))
		}

		return ungo.None[error]()

	case "!close":
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected index of workspace"))
		}

		idx, err := strconv.ParseUint(command[1], 10, 64)

		if err != nil {
			return ungo.Some(fmt.Errorf("could not parse index: %v", err))
		}

		if idx >= uint64(state.workspaces.Size()) {
			return ungo.Some(fmt.Errorf("workspace at %d does not exist.", idx))
		}

		if idx == uint64(state.cur_workspace) {
			return ungo.Some(fmt.Errorf("You can not close a workspace you are currently in."))
		}

		if idx < uint64(state.cur_workspace) {
			state.cur_workspace--
		}

		state.workspaces.Remove(int(idx))
		return ungo.None[error]()

	case "!drop":
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected name of scope"))
		}
		var found_scope bool = false
		state.workspaces.Get(state.cur_workspace).IfPresent(func(w *Workspace) {
			w.scopes.ForEach(func(idx int, sc *Scope) {
				if sc.name == command[1] {
					w.scopes.Get(idx).Value().overrides.Clear()
					w.scopes.Remove(idx)
					found_scope = true
					return
				}
			})

		})

		if !found_scope {
			return ungo.Some(fmt.Errorf("could not find scope with name '%s'", command[1]))
		}

		return ungo.None[error]()

	case "!set":
		if len(command) < 3 {
			return ungo.Some(fmt.Errorf("expected name of field and value"))
		}

		if state.workspaces.Get(state.cur_workspace).Value().scopes.Size() == 0 {
			return ungo.Some(fmt.Errorf("no scopes currently open"))
		}
		scopes := state.workspaces.Get(state.cur_workspace).Value().scopes

		scopes.Get(scopes.Size() - 1).IfPresent(func(s *Scope) {
			s.overrides.Set(command[1], command[2])
		})

		return ungo.None[error]()

	case "!wear":
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected .cloth file path"))
		}

		data, err := os.ReadFile(command[1])

		if err != nil {
			return ungo.Some(fmt.Errorf("failed to read file '%s': %v", command[1], err))
		}

		lines := strings.Split(string(data), "\n")

		for _, line := range lines {
			RunString(state, line)
		}

		return ungo.None[error]()

	default:
		return ungo.Some(fmt.Errorf("unrecognised internal command: %s", command[0]))
	}
}
