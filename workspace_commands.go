package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/loeredami/ungo"
)

type StateCmd func(state *State, args []string) ungo.Optional[error]

var StateCommands = ungo.NewSmallMap[string, StateCmd](256)

func RegisterCmd(name string, fn StateCmd) {
	StateCommands.Set(name, fn)
}

func HandleStateCommands(state *State, command []string) ungo.Optional[error] {
	if cmd, exists := StateCommands.Get(command[0]); exists {
		return cmd(state, command)
	}
	return ungo.Some(fmt.Errorf("unrecognized internal command: %s", command[0]))
}

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

func init() {
	RegisterCmd("!new", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected argument 'w' for workspace or 's' for scope"))
		}

		switch command[1] {
		case "w":
			state.workspaces.Add(&Workspace{
				name:   "",
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
	})
	RegisterCmd("!switch", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected index or label of workspace"))
		}

		var found_label bool = false
		state.workspaces.ForEach(func(idx int, ws *Workspace) {
			if found_label {
				return
			}
			if ws.name == command[1] {
				state.cur_workspace = int(idx)
				found_label = true
			}
		})

		if found_label {
			return ungo.None[error]()
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
	})

	RegisterCmd("!close", func(state *State, command []string) ungo.Optional[error] {
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
	})

	RegisterCmd("!clone", func(state *State, command []string) ungo.Optional[error] {
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

		state.workspaces.Add(state.workspaces.Get(int(idx)).Value().Clone())
		return ungo.None[error]()
	})

	RegisterCmd("!drop", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected name of scope"))
		}
		var found_scope bool = false
		state.workspaces.Get(state.cur_workspace).IfPresent(func(w *Workspace) {
			w.scopes.ForEach(func(idx int, sc *Scope) {
				if found_scope {
					return
				}
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
	})
	RegisterCmd("!set", func(state *State, command []string) ungo.Optional[error] {
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
	})
	RegisterCmd("!wear", func(state *State, command []string) ungo.Optional[error] {
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
	})
	RegisterCmd("!color", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 3 {
			return ungo.Some(fmt.Errorf("expected field name <string> and color code <int>"))
		}

		color_int, err := strconv.ParseUint(command[2], 10, 64)
		if err != nil {
			return ungo.Some(fmt.Errorf("could not parse color int '%s': %v", command[1], err))
		}

		color := fmt.Sprintf("\033[%dm", color_int)

		switch command[1] {
		case "err":
			state.config.ErrorCol = color
			return ungo.None[error]()
		case "ls-dir":
			state.config.LSDirCol = color
			return ungo.None[error]()
		case "ls-sym-link":
			state.config.LSSymLinkCol = color
			return ungo.None[error]()
		case "ls-exec":
			state.config.LSExecCol = color
			return ungo.None[error]()
		case "ls-normal":
			state.config.LSNormalCol = color
			return ungo.None[error]()
		case "sudo-prompt":
			state.config.SudoPromptCol = color
			return ungo.None[error]()
		case "prompt":
			state.config.PromptCol = color
			return ungo.None[error]()
		case "idx":
			state.config.IdxCol = color
			return ungo.None[error]()
		case "cur-ws":
			state.config.CurWSCol = color
			return ungo.None[error]()
		case "cur-dir":
			state.config.CurDirCol = color
			return ungo.None[error]()
		case "cur-dir-indic":
			state.config.CurDirIndicCol = color
			return ungo.None[error]()
		case "git-branch":
			state.config.GitBranchCol = color
			return ungo.None[error]()
		case "time":
			state.config.TimeCol = color
			return ungo.None[error]()
		case "time-prefix":
			state.config.TimePrefixCol = color
			return ungo.None[error]()
		case "scope":
			state.config.ScopeCol = color
			return ungo.None[error]()
		case "input":
			state.config.InputCol = color
			return ungo.None[error]()
		case "path":
			state.config.PathCol = color
			return ungo.None[error]()
		case "input-string":
			state.config.InputStringCol = color
			return ungo.None[error]()
		case "input-num":
			state.config.InputNumCol = color
			return ungo.None[error]()
		case "input-path":
			state.config.InputPathCol = color
			return ungo.None[error]()
		case "input-var":
			state.config.InputVarCol = color
			return ungo.None[error]()
		case "input-brace":
			state.config.InputBraceCol = color
			return ungo.None[error]()
		case "ghost":
			state.config.GhostCol = color
			return ungo.None[error]()
		case "workspace":
			state.config.WorkspaceNameCol = color
			return ungo.None[error]()
		}

		return ungo.Some(fmt.Errorf("cound not find color field '%s'", command[1]))
	})
	RegisterCmd("!local", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 3 {
			return ungo.Some(fmt.Errorf("expected field name <string> and string value <string>"))
		}

		switch command[1] {
		case "sudo-prompt":
			state.config.SudoPrompt = command[2]
			return ungo.None[error]()
		case "prompt":
			state.config.Prompt = command[2]
			return ungo.None[error]()
		case "scope-sign":
			state.config.ScopeSign = command[2]
			return ungo.None[error]()
		}

		return ungo.Some(fmt.Errorf("cound not find string field '%s'", command[1]))
	})
	RegisterCmd("!label", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected label for workspace"))
		}

		state.workspaces.Get(state.cur_workspace).Value().name = command[1]

		return ungo.None[error]()
	})
	RegisterCmd("!enable-colors", func(state *State, command []string) ungo.Optional[error] {
		state.config.ColorMode = true
		return ungo.None[error]()
	})
	RegisterCmd("!disable-colors", func(state *State, command []string) ungo.Optional[error] {
		state.config.ColorMode = false
		return ungo.None[error]()
	})
	RegisterCmd("!reset", func(state *State, command []string) ungo.Optional[error] {
		state.ResetConfig()
		return ungo.None[error]()
	})
	RegisterCmd("!snapshot", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected .cloth file"))
		}
		scopes := state.workspaces.Get(state.cur_workspace).Value().scopes

		scope := scopes.Get(scopes.Size() - 1)

		failure := ungo.None[error]()

		scope.IfPresent(func(s *Scope) {
			err := os.WriteFile(command[1], s.Encode(), 0644)
			if err != nil {
				failure = ungo.Some(err)
			}
		})

		scope.IfAbsent(func(s **Scope) {
			failure = ungo.Some(fmt.Errorf("could not find any open scope"))
		})

		return failure
	})
	RegisterCmd("!snapshot-ws", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected .cloth file"))
		}
		ws := state.workspaces.Get(state.cur_workspace).Value()

		failure := ungo.None[error]()
		err := os.WriteFile(command[1], ws.Encode(), 0644)
		if err != nil {
			failure = ungo.Some(err)
		}
		return failure
	})

}
