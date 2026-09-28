package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/loeredami/ungo"
)

type StateCmd func(state *State, args []string) ungo.Optional[error]

var StateCommands = ungo.NewSmallMap[string, StateCmd](64)

func RegisterCmd(name string, fn StateCmd) {
	StateCommands.Set(name, fn)
}

func HandleStateCommands(state *State, command []string) ungo.Optional[error] {
	if cmd, exists := StateCommands.Get(command[0]); exists {
		return cmd(state, command)
	}
	return ungo.Some(fmt.Errorf("unrecognized internal command: %s", command[0]))
}

func currentWorkspace(state *State) (*Workspace, error) {
	if state == nil || state.workspaces == nil {
		return nil, fmt.Errorf("no current workspace")
	}
	workspace := state.workspaces.Get(state.cur_workspace)
	if !workspace.HasValue() || workspace.Value() == nil {
		return nil, fmt.Errorf("current workspace at index %d does not exist", state.cur_workspace)
	}
	current := workspace.Value()
	if current.scopes == nil {
		return nil, fmt.Errorf("current workspace at index %d has no scope list", state.cur_workspace)
	}
	return current, nil
}

func GetEnvValue(state *State, key string) ungo.Optional[[]string] {
	if state == nil || state.workspaces == nil {
		return ungo.None[[]string]()
	}
	wsOpt := state.workspaces.Get(state.cur_workspace)
	if !wsOpt.HasValue() || wsOpt.Value() == nil || wsOpt.Value().scopes == nil {
		return ungo.None[[]string]()
	}
	ws := wsOpt.Value()

	override := ""
	hasOverride := false
	ws.scopes.ForEach(func(_ int, scope *Scope) {
		if value, ok := scope.overrides.Get(key); ok {
			override = value
			hasOverride = true
		}
	})
	if hasOverride {
		tokens := Lex(override)
		commandStrings := []string{}
		tokens.ForEach(func(_ int, token Token) {
			if token.Type == EndOfInput {
				return
			}
			if token.Type == Path {
				token.Value.IfPresent(func(value string) {
					commandStrings = append(commandStrings, UnformatPathIfInHome(value))
				})
				return
			}
			token.Value.IfPresent(func(value string) {
				commandStrings = append(commandStrings, value)
			})
		})
		return ungo.Some(commandStrings)
	}

	value, exists := os.LookupEnv(key)
	if !exists {
		return ungo.None[[]string]()
	}
	result := []string{}
	found := false
	Lex(value).ForEach(func(_ int, token Token) {
		if found {
			return
		}
		result = append(result, token.Value.OrElse(""))
		found = true
	})
	return ungo.Some(result)
}

func writeLastStatus(state *State, w io.Writer) {
	fmt.Fprintln(w, state.lastExitCode)
}

func writeSecurityStatus(state *State, w io.Writer) error {
	path := state.configPath
	if path == "" {
		uDir, err := os.UserConfigDir()
		if err != nil {
			return fmt.Errorf("identify default configuration directory: %w", err)
		}
		path = filepath.Join(uDir, ".loin", "default.cloth")
	}

	fmt.Fprintf(w, "Configuration path: %s\n", path)
	fmt.Fprintf(w, "Configuration source: %s\n", state.configSource)
	trusted := "No"
	if state.configSource == SourceDefaultCloth {
		trusted = "Yes"
	}
	fmt.Fprintf(w, "Configuration trusted: %s\n", trusted)
	gray := "No"
	if state.configSource.grayListed() {
		gray = "Yes"
	}
	fmt.Fprintf(w, "Configuration gray-listed: %s\n", gray)

	interactive, defaultCloth := 0, 0
	for _, entry := range state.trustStore.Entries() {
		switch entry.Source {
		case SourceInteractive:
			interactive++
		case SourceDefaultCloth:
			defaultCloth++
		}
	}
	fmt.Fprintf(w, "Session trust rules: %d (not saved between Loin launches)\n", len(state.trustStore.Entries()))
	fmt.Fprintf(w, "  interactive: %d\n", interactive)
	fmt.Fprintf(w, "  default.cloth: %d\n", defaultCloth)
	fmt.Fprintln(w, "Privilege state: normal (explicit elevation is not active)")
	fmt.Fprintln(w, "Note: scopes manage environment overrides and workspace state, not executable trust.")
	return nil
}

func init() {
	RegisterCmd("!last-status", func(state *State, command []string) ungo.Optional[error] {
		writeLastStatus(state, os.Stdout)
		return ungo.None[error]()
	})

	RegisterCmd("!security-status", func(state *State, command []string) ungo.Optional[error] {
		if err := writeSecurityStatus(state, os.Stdout); err != nil {
			return ungo.Some(err)
		}
		return ungo.None[error]()
	})

	RegisterCmd("!trust", func(state *State, command []string) ungo.Optional[error] {
		fromProtectedDefault := state.commandSource == SourceDefaultCloth
		fromInteractive := state.commandSource == SourceInteractive && state.interactiveInput
		if !fromProtectedDefault && !fromInteractive {
			return ungo.Some(fmt.Errorf("!trust requires direct interactive input"))
		}
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected executable path, basename, or explicit glob"))
		}
		if strings.HasPrefix(command[1], "!") {
			return ungo.Some(fmt.Errorf("workspace commands cannot be trusted as executables"))
		}
		kind, rule := ParseTrustRule(command[1])
		if fromInteractive {
			fmt.Fprintf(os.Stderr, "Trust rule %q for this session (%s)? [y/N]: ", rule, kind)
			var confirmation string
			if _, err := fmt.Fscanln(os.Stdin, &confirmation); err != nil || !strings.EqualFold(confirmation, "y") && !strings.EqualFold(confirmation, "yes") {
				fmt.Fprintln(os.Stderr, "trust entry not added")
				return ungo.None[error]()
			}
		}

		source := SourceInteractive
		if fromProtectedDefault {
			source = SourceDefaultCloth
		}
		entry := TrustEntry{Rule: rule, Kind: kind, Source: source}
		state.trustStore.Add(entry)
		fmt.Printf("trusted %s for this session from %s (%s)\n", rule, source, kind)
		return ungo.None[error]()
	})

	RegisterCmd("!trust-list", func(state *State, command []string) ungo.Optional[error] {
		if state.commandSource != SourceInteractive || !state.interactiveInput {
			return ungo.Some(fmt.Errorf("!trust-list requires direct interactive input"))
		}
		entries := state.trustStore.Entries()
		if len(entries) == 0 {
			fmt.Println("session trust list is empty")
			return ungo.None[error]()
		}
		fmt.Println("Session trust rules (active until exit):")
		for _, entry := range entries {
			fmt.Printf("%s (%s, source: %s)\n", entry.Rule, entry.Kind, entry.Source)
		}
		return ungo.None[error]()
	})

	RegisterCmd("!untrust", func(state *State, command []string) ungo.Optional[error] {
		if state.commandSource != SourceInteractive || !state.interactiveInput {
			return ungo.Some(fmt.Errorf("!untrust requires direct interactive input"))
		}
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected executable path, basename, or explicit glob"))
		}
		kind, rule := ParseTrustRule(command[1])
		if !state.trustStore.RemoveRule(kind, rule) {
			return ungo.Some(fmt.Errorf("trust entry not found: %s", rule))
		}
		fmt.Printf("removed trust entry %s\n", rule)
		return ungo.None[error]()
	})

	RegisterCmd("!new", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected argument 'w' for workspace or 's' for scope"))
		}

		switch command[1] {
		case "w":
			current, err := currentWorkspace(state)
			if err != nil {
				return ungo.Some(err)
			}
			state.workspaces.Add(&Workspace{
				name:   "",
				path:   current.path,
				scopes: ungo.NewLinkedList[*Scope](),
			})
		case "s":
			if len(command) < 3 {
				return ungo.Some(fmt.Errorf("error creating scope: no name given"))
			}
			current, err := currentWorkspace(state)
			if err != nil {
				return ungo.Some(err)
			}
			scope := &Scope{
				name:      command[2],
				overrides: ungo.NewSmallMap[string, string](256),
			}
			current.scopes.Add(scope)
		default:
			return ungo.Some(fmt.Errorf("expected argument 'workspace'"))
		}

		return ungo.None[error]()
	})
	RegisterCmd("!switch", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected index or label of workspace"))
		}
		if state.workspaces == nil {
			return ungo.Some(fmt.Errorf("no workspaces are available"))
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

		workspace := state.workspaces.Get(state.cur_workspace)
		if !workspace.HasValue() || workspace.Value() == nil {
			state.cur_workspace = prev
			return ungo.Some(fmt.Errorf("workspace at %d does not exist", idx))
		}
		err = os.Chdir(workspace.Value().path)

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
		if state.workspaces == nil {
			return ungo.Some(fmt.Errorf("no workspaces are available"))
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
		if state.workspaces == nil {
			return ungo.Some(fmt.Errorf("no workspaces are available"))
		}

		idx, err := strconv.ParseUint(command[1], 10, 64)

		if err != nil {
			return ungo.Some(fmt.Errorf("could not parse index: %v", err))
		}

		if idx >= uint64(state.workspaces.Size()) {
			return ungo.Some(fmt.Errorf("workspace at %d does not exist.", idx))
		}

		workspace := state.workspaces.Get(int(idx))
		if !workspace.HasValue() || workspace.Value() == nil {
			return ungo.Some(fmt.Errorf("workspace at %d does not exist", idx))
		}
		state.workspaces.Add(workspace.Value().Clone())
		return ungo.None[error]()
	})

	RegisterCmd("!drop", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 2 {
			return ungo.Some(fmt.Errorf("expected name of scope"))
		}
		var found_scope bool = false
		workspace, err := currentWorkspace(state)
		if err != nil {
			return ungo.Some(err)
		}
		workspace.scopes.ForEach(func(idx int, sc *Scope) {
			if found_scope {
				return
			}
			if sc.name == command[1] {
				sc.overrides.Clear()
				workspace.scopes.Remove(idx)
				found_scope = true
			}
		})

		if !found_scope {
			return ungo.Some(fmt.Errorf("could not find scope with name '%s'", command[1]))
		}

		return ungo.None[error]()
	})
	RegisterCmd("!set-ifn", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 3 {
			return ungo.Some(fmt.Errorf("expected name of field and value"))
		}

		workspace, err := currentWorkspace(state)
		if err != nil {
			return ungo.Some(err)
		}
		if workspace.scopes.Size() == 0 {
			return ungo.Some(fmt.Errorf("no scopes currently open"))
		}
		scopes := workspace.scopes

		GetEnvValue(state, command[1]).IfAbsent(func(*[]string) {
			scopes.Get(scopes.Size() - 1).IfPresent(func(s *Scope) {
				s.overrides.Set(command[1], command[2])
			})
		})

		return ungo.None[error]()
	})
	RegisterCmd("!set", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 3 {
			return ungo.Some(fmt.Errorf("expected name of field and value"))
		}

		workspace, err := currentWorkspace(state)
		if err != nil {
			return ungo.Some(err)
		}
		if workspace.scopes.Size() == 0 {
			return ungo.Some(fmt.Errorf("no scopes currently open"))
		}
		scopes := workspace.scopes

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
			RunStringFromSource(state, line, SourceClothFile)
		}

		return ungo.None[error]()
	})
	RegisterCmd("!color", func(state *State, command []string) ungo.Optional[error] {
		if len(command) < 3 {
			return ungo.Some(fmt.Errorf("expected field name <string> and one or more color codes <int>"))
		}

		color_codes := strings.Split(strings.Join(command[2:], ";"), ";")
		var color strings.Builder
		for _, code := range color_codes {
			if _, err := strconv.ParseUint(code, 10, 64); err != nil {
				return ungo.Some(fmt.Errorf("could not parse color code '%s': %v", code, err))
			}
			color.WriteString(fmt.Sprintf("\033[%sm", code))
		}

		switch command[1] {
		case "err":
			state.config.ErrorCol = color.String()
			return ungo.None[error]()
		case "ls-dir":
			state.config.LSDirCol = color.String()
			return ungo.None[error]()
		case "ls-sym-link":
			state.config.LSSymLinkCol = color.String()
			return ungo.None[error]()
		case "ls-exec":
			state.config.LSExecCol = color.String()
			return ungo.None[error]()
		case "ls-normal":
			state.config.LSNormalCol = color.String()
			return ungo.None[error]()
		case "sudo-prompt":
			state.config.SudoPromptCol = color.String()
			return ungo.None[error]()
		case "prompt":
			state.config.PromptCol = color.String()
			return ungo.None[error]()
		case "idx":
			state.config.IdxCol = color.String()
			return ungo.None[error]()
		case "cur-ws":
			state.config.CurWSCol = color.String()
			return ungo.None[error]()
		case "cur-dir":
			state.config.CurDirCol = color.String()
			return ungo.None[error]()
		case "cur-dir-indic":
			state.config.CurDirIndicCol = color.String()
			return ungo.None[error]()
		case "git-branch":
			state.config.GitBranchCol = color.String()
			return ungo.None[error]()
		case "time":
			state.config.TimeCol = color.String()
			return ungo.None[error]()
		case "time-prefix":
			state.config.TimePrefixCol = color.String()
			return ungo.None[error]()
		case "scope":
			state.config.ScopeCol = color.String()
			return ungo.None[error]()
		case "input":
			state.config.InputCol = color.String()
			return ungo.None[error]()
		case "path":
			state.config.PathCol = color.String()
			return ungo.None[error]()
		case "input-string":
			state.config.InputStringCol = color.String()
			return ungo.None[error]()
		case "input-num":
			state.config.InputNumCol = color.String()
			return ungo.None[error]()
		case "input-path":
			state.config.InputPathCol = color.String()
			return ungo.None[error]()
		case "input-var":
			state.config.InputVarCol = color.String()
			return ungo.None[error]()
		case "input-brace":
			state.config.InputBraceCol = color.String()
			return ungo.None[error]()
		case "ghost":
			state.config.GhostCol = color.String()
			return ungo.None[error]()
		case "workspace":
			state.config.WorkspaceNameCol = color.String()
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

		workspace, err := currentWorkspace(state)
		if err != nil {
			return ungo.Some(err)
		}
		workspace.name = command[1]

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
		workspace, err := currentWorkspace(state)
		if err != nil {
			return ungo.Some(err)
		}
		scopes := workspace.scopes

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
		ws, err := currentWorkspace(state)
		if err != nil {
			return ungo.Some(err)
		}

		failure := ungo.None[error]()
		err = os.WriteFile(command[1], ws.Encode(), 0644)
		if err != nil {
			failure = ungo.Some(err)
		}
		return failure
	})

}
