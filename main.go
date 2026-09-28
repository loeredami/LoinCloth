package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	terminal "github.com/wayneashleyberry/terminal-dimensions"

	"github.com/loeredami/ungo"
)

func ReformatPathIfInHome(path string) string {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, dirname) {
		path = strings.Replace(path, dirname, "~", 1)
	}
	return path
}

func UnformatPathIfInHome(path string) string {
	dirname, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, "~") {
		path = strings.Replace(path, "~", dirname, 1)
	}
	return path
}

func terminalTextWidth(text string) int {
	width := 0
	inEscape := false
	for _, r := range text {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		if r == '\u200d' || r == '\ufe0e' || r == '\ufe0f' || unicode.Is(unicode.Mn, r) {
			continue
		}
		if r >= 0x1100 && (r <= 0x115f || r == 0x2329 || r == 0x232a ||
			(r >= 0x2e80 && r <= 0xa4cf) || (r >= 0xac00 && r <= 0xd7a3) ||
			(r >= 0xf900 && r <= 0xfaff) || (r >= 0xfe10 && r <= 0xfe19) ||
			(r >= 0xfe30 && r <= 0xfe6f) || (r >= 0xff00 && r <= 0xff60) ||
			(r >= 0xffe0 && r <= 0xffe6)) {
			width += 2
		} else {
			width++
		}
	}
	return width
}

func (state *State) RefreshLine(prompt string, buffer []rune, cursor int) {
	termWidth, err := terminal.Width()
	if termWidth <= 0 || err != nil {
		termWidth = 80
	}

	visiblePromptLen := terminalTextWidth(prompt)

	highlightedBuffer := state.highlightInput(buffer)

	displayText := string(buffer)
	if cursor == len(buffer) {
		displayText += state.ghostSuggestion
	}

	totalWidth := visiblePromptLen + terminalTextWidth(displayText)
	termWidthInt := int(termWidth)
	rowCount := (totalWidth + termWidthInt - 1) / termWidthInt
	if rowCount == 0 {
		rowCount = 1
	}

	if state.lastRowCount > 1 {
		fmt.Printf("\033[%dA", state.lastRowCount-1)
	}
	fmt.Print("\r\033[J")

	fmt.Printf("%s%s", prompt, highlightedBuffer)

	if cursor == len(buffer) && state.ghostSuggestion != "" {
		fmt.Printf("%s%s%s", state.GetColor(state.config.GhostCol), state.ghostSuggestion, state.Reset())
	}

	targetPos := visiblePromptLen + terminalTextWidth(string(buffer[:cursor]))
	currentPos := visiblePromptLen + terminalTextWidth(string(buffer))
	if cursor == len(buffer) {
		currentPos += terminalTextWidth(state.ghostSuggestion)
	}

	moveBack := currentPos - targetPos
	if moveBack > 0 {
		fmt.Printf("\033[%dD", moveBack)
	}

	state.lastRowCount = int(rowCount)
}

func (state *State) HandleAutocomplete(buffer []rune, cursor *int) []rune {
	if state.lastWasTab && len(state.autoCompleteMatches) > 0 {
		buffer = append(buffer[:*cursor-state.lastAddedLen], buffer[*cursor:]...)
		*cursor -= state.lastAddedLen
		state.autoCompleteIndex = (state.autoCompleteIndex + 1) % len(state.autoCompleteMatches)
	} else {
		state.autoCompleteMatches = []string{}
		state.autoCompleteIndex = 0
		state.lastAddedLen = 0

		currentLine := string(buffer[:*cursor])

		if strings.HasPrefix(currentLine, "!") {
			StateCommands.ForEach(func(cmdName string, cmd StateCmd) {
				if strings.HasPrefix(cmdName, currentLine) {
					fmt.Print("\a")
					state.autoCompleteMatches = append(state.autoCompleteMatches, cmdName[len(currentLine):]+" ")
				}
			})
		}

		if len(state.autoCompleteMatches) == 0 {
			lastWordStart := strings.LastIndexAny(currentLine, " \t\n\r\"'{}/\\") + 1
			lastWord := currentLine[lastWordStart:]

			if strings.HasPrefix(lastWord, "$") {
				varNamePart := lastWord[1:]
				seen := make(map[string]bool)

				state.workspaces.Get(state.cur_workspace).IfPresent(func(ws *Workspace) {
					ws.scopes.ForEach(func(idx int, sc *Scope) {
						sc.overrides.ForEach(func(key, value string) {
							if strings.HasPrefix(key, varNamePart) && !seen[key] {
								state.autoCompleteMatches = append(state.autoCompleteMatches, key[len(varNamePart):])
								seen[key] = true
							}
						})
					})
				})

				for _, e := range os.Environ() {
					pair := strings.SplitN(e, "=", 2)
					if strings.HasPrefix(pair[0], varNamePart) && !seen[pair[0]] {
						state.autoCompleteMatches = append(state.autoCompleteMatches, pair[0][len(varNamePart):])
						seen[pair[0]] = true
					}
				}
			}
		}

		if len(state.autoCompleteMatches) == 0 {
			var lastArgUnescaped strings.Builder
			inQuotes, escaped := false, false

			for _, r := range currentLine {
				if escaped {
					lastArgUnescaped.WriteRune(r)
					escaped = false
					continue
				}
				if r == '\\' {
					escaped = true
					continue
				}
				if r == '"' {
					inQuotes = !inQuotes
					continue
				}
				if r == ' ' && !inQuotes {
					lastArgUnescaped.Reset()
					continue
				}
				lastArgUnescaped.WriteRune(r)
			}

			searchPath := lastArgUnescaped.String()
			resolvedSearchPath := UnformatPathIfInHome(searchPath)

			dir, prefix := ".", searchPath
			lastSlash := strings.LastIndex(resolvedSearchPath, "/")

			if lastSlash != -1 {
				dir = resolvedSearchPath[:lastSlash]
				if dir == "" {
					dir = "/"
				}
				lastSlashInOriginal := strings.LastIndex(searchPath, "/")
				prefix = searchPath[lastSlashInOriginal+1:]
			} else if strings.HasPrefix(searchPath, "~") {
				dir = UnformatPathIfInHome("~")
				prefix = ""
			}

			entries, err := os.ReadDir(dir)
			if err == nil {
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), prefix) {
						remainder := entry.Name()[len(prefix):]
						var appendStr strings.Builder

						for _, r := range remainder {
							if r == ' ' && !inQuotes {
								appendStr.WriteRune('\\')
							}
							appendStr.WriteRune(r)
						}

						if entry.IsDir() {
							appendStr.WriteRune('/')
						} else {
							if inQuotes {
								appendStr.WriteRune('"')
							}
							appendStr.WriteRune(' ')
						}
						state.autoCompleteMatches = append(state.autoCompleteMatches, appendStr.String())
					}
				}
			}
		}
	}

	if len(state.autoCompleteMatches) > 0 {
		match := state.autoCompleteMatches[state.autoCompleteIndex]
		newSuffix := []rune(match)
		buffer = append(buffer[:*cursor], append(newSuffix, buffer[*cursor:]...)...)
		*cursor += len(newSuffix)
		state.lastAddedLen = len(newSuffix)
	}

	return buffer
}

func (state *State) PrettyLS(w io.Writer, cmdArgs []string) int {
	path := "."
	if len(cmdArgs) > 1 {
		path = cmdArgs[1]
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(w, "%sError: %v%s\n", Red, err, Reset)
		return 1
	}

	fmt.Fprintln(w)
	for i, entry := range entries {
		name := entry.Name()
		style := state.GetColor(state.config.LSNormalCol)
		indicator := ""

		if entry.IsDir() {
			style = state.GetColor(state.config.LSDirCol)
			indicator = "/"
		} else if entry.Type()&os.ModeSymlink != 0 {
			style = state.GetColor(state.config.LSSymLinkCol)
			indicator = "@"
		} else if info, err := entry.Info(); err == nil && info.Mode()&0111 != 0 {
			style = state.GetColor(state.config.LSExecCol)
			indicator = "*"
		}

		fmt.Fprintf(w, "%s%-20s%s", style, name+indicator, state.Reset())

		if (i+1)%4 == 0 {
			fmt.Fprintln(w)
		}
	}
	fmt.Fprint(w, "\n\n")
	return 0
}

func Prompt(state *State, time_taken ungo.Optional[time.Duration]) string {
	promptStr := renderPromptInfo(state, time_taken)

	fd := os.Stdin.Fd()
	oldState, err := MakeRaw(fd)
	if err != nil {
		var input string
		fmt.Scanln(&input)
		return input
	}
	defer RestoreTerminal(fd, oldState)

	return readRawInput(state, promptStr)
}

func RunAndCapture(state *State, cmdArgs []string) string {
	var builder strings.Builder
	Run(state, cmdArgs, &builder)
	return builder.String()
}

func processTokens(state *State, tokenSlice []Token) []string {
	var cmd []string

	for i := 0; i < len(tokenSlice); i++ {
		token := tokenSlice[i]
		if token.Type == EndOfInput {
			break
		}
		if token.Type == CloseBrace {
			continue
		}

		if token.Type == OpenBrace {
			depth := 1
			var innerTokens []Token
			j := i + 1
			for ; j < len(tokenSlice); j++ {
				if tokenSlice[j].Type == OpenBrace {
					depth++
				} else if tokenSlice[j].Type == CloseBrace {
					depth--
					if depth == 0 {
						break
					}
				}
				innerTokens = append(innerTokens, tokenSlice[j])
			}

			if depth == 0 {
				i = j
			} else {
				i = len(tokenSlice)
			}

			if len(innerTokens) > 0 {
				output := RunTokenPipelineAndCapture(state, innerTokens)
				words := strings.Fields(output)
				cmd = append(cmd, words...)
			}
			continue
		}

		token.Value.IfPresent(func(val string) {
			if token.Type == Path {
				val = UnformatPathIfInHome(val)

				if strings.Contains(val, "*") {
					matches, err := filepath.Glob(val)
					if err == nil && len(matches) > 0 {
						cmd = append(cmd, matches...)
						return
					}
				}
			}

			if token.Type == Varname {
				env_val_opt := GetEnvValue(state, val)
				if env_val_opt.HasValue() {
					env_val := env_val_opt.Value()
					cmd = append(cmd, env_val...)
					return
				}
			}

			cmd = append(cmd, val)
		})
	}
	return cmd
}

type PipelineCommand struct {
	args       []string
	stdinPath  string
	stdoutPath string
	appendOut  bool
}

func parsePipeline(state *State, tokens []Token) ([]PipelineCommand, error) {
	commands := []PipelineCommand{}
	current := []Token{}
	var stdinPath, stdoutPath string
	appendOut := false
	stdinRedirected, stdoutRedirected := false, false
	braceDepth := 0
	afterPipe := false

	flush := func() error {
		args := processTokens(state, current)
		if len(args) == 0 || args[0] == "" {
			return fmt.Errorf("expected a command")
		}
		commands = append(commands, PipelineCommand{
			args:       args,
			stdinPath:  stdinPath,
			stdoutPath: stdoutPath,
			appendOut:  appendOut,
		})
		current = nil
		stdinPath = ""
		stdoutPath = ""
		appendOut = false
		stdinRedirected, stdoutRedirected = false, false
		return nil
	}

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token.Type == EndOfInput {
			break
		}
		if token.Type == CloseBrace && braceDepth == 0 {
			return nil, fmt.Errorf("unmatched closing brace")
		}
		if braceDepth > 0 {
			current = append(current, token)
			if token.Type == OpenBrace {
				braceDepth++
			} else if token.Type == CloseBrace {
				braceDepth--
			}
			continue
		}

		switch token.Type {
		case Pipe:
			if len(current) == 0 {
				return nil, fmt.Errorf("pipe has no command before it")
			}
			if err := flush(); err != nil {
				return nil, err
			}
			afterPipe = true
		case RedirectIn, RedirectOut, RedirectAppend:
			if len(current) == 0 && len(commands) > 0 {
				return nil, fmt.Errorf("redirection has no command in this pipeline stage")
			}
			if i+1 >= len(tokens) || tokens[i+1].Type == EndOfInput {
				return nil, fmt.Errorf("redirection requires a path")
			}
			switch tokens[i+1].Type {
			case Identifier, String, Number, Path, Varname:
			default:
				return nil, fmt.Errorf("redirection requires a path")
			}
			pathTokens := []Token{tokens[i+1], {Type: EndOfInput}}
			paths := processTokens(state, pathTokens)
			if len(paths) != 1 || paths[0] == "" {
				return nil, fmt.Errorf("redirection requires one path")
			}
			switch token.Type {
			case RedirectIn:
				if stdinRedirected {
					return nil, fmt.Errorf("input redirection may only be specified once per command")
				}
				stdinPath = paths[0]
				stdinRedirected = true
			case RedirectOut:
				if stdoutRedirected {
					return nil, fmt.Errorf("output redirection may only be specified once per command")
				}
				stdoutPath = paths[0]
				appendOut = false
				stdoutRedirected = true
			case RedirectAppend:
				if stdoutRedirected {
					return nil, fmt.Errorf("output redirection may only be specified once per command")
				}
				stdoutPath = paths[0]
				appendOut = true
				stdoutRedirected = true
			}
			i++
		default:
			current = append(current, token)
			afterPipe = false
			if token.Type == OpenBrace {
				braceDepth++
			}
		}
	}
	if afterPipe {
		return nil, fmt.Errorf("pipe has no command after it")
	}
	if braceDepth != 0 {
		return nil, fmt.Errorf("unclosed brace in command")
	}
	if len(current) > 0 {
		if err := flush(); err != nil {
			return nil, err
		}
	}
	if len(commands) == 0 {
		return nil, nil
	}
	return commands, nil
}

func commandEnvironment(state *State) []string {
	environment := os.Environ()
	overrideCount := 0
	if state != nil {
		state.workspaces.Get(state.cur_workspace).IfPresent(func(ws *Workspace) {
			ws.scopes.ForEach(func(idx int, s *Scope) {
				overrideCount += s.overrides.Size()
			})
		})
	}

	envMap := ungo.NewSmallMap[string, string](len(environment) + overrideCount)
	for _, entry := range environment {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			envMap.Set(key, value)
		}
	}
	if state != nil {
		state.workspaces.Get(state.cur_workspace).IfPresent(func(ws *Workspace) {
			ws.scopes.ForEach(func(_ int, scope *Scope) {
				scope.overrides.ForEach(func(key, value string) {
					envMap.Set(key, value)
				})
			})
		})
	}

	finalEnv := make([]string, 0, envMap.Size())
	envMap.ForEach(func(key, value string) {
		finalEnv = append(finalEnv, key+"="+value)
	})
	return finalEnv
}

func RunTokenPipelineAndCapture(state *State, tokens []Token) string {
	commands, err := parsePipeline(state, tokens)
	if err != nil {
		return err.Error()
	}
	if len(commands) == 0 {
		return ""
	}
	var output strings.Builder
	status := runPipeline(state, commands, &output)
	if state != nil {
		state.lastExitCode = status
	}
	return output.String()
}

func isPipelineInternal(name string) bool {
	if name == "ls" || name == "cd" || strings.HasPrefix(name, "!") {
		return true
	}
	if !is_windows {
		return false
	}
	switch name {
	case "mkdir", "clear", "echo", "cp", "mv", "rm":
		return true
	default:
		return false
	}
}

func runBufferedPipeline(state *State, commands []PipelineCommand, output io.Writer) int {
	var input io.Reader = os.Stdin
	var inputFile *os.File
	if commands[0].stdinPath != "" {
		file, err := os.Open(commands[0].stdinPath)
		if err != nil {
			fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
			return 1
		}
		inputFile = file
		defer inputFile.Close()
		input = file
	}

	for i, command := range commands {
		if command.stdinPath != "" && i > 0 {
			file, err := os.Open(command.stdinPath)
			if err != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
				return 1
			}
			data, readErr := io.ReadAll(file)
			file.Close()
			if readErr != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), readErr, state.Reset())
				return 1
			}
			input = bytes.NewReader(data)
		}
		if command.args[0] == "cd" {
			fmt.Fprintf(output, "%scd cannot be used in a pipeline%s\n", state.GetColor(state.config.ErrorCol), state.Reset())
			return 1
		}

		var stageOutput bytes.Buffer
		stageStatus := 0
		if isPipelineInternal(command.args[0]) && command.args[0] != "ls" {
			stageStatus = Run(state, command.args, &stageOutput)
		} else if command.args[0] == "ls" {
			path := "."
			if len(command.args) > 1 {
				path = command.args[1]
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
				return 1
			}
			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() {
					name += "/"
				}
				fmt.Fprintln(&stageOutput, name)
			}
		} else {
			path, err := exec.LookPath(command.args[0])
			if err != nil {
				fmt.Fprintf(output, "%sCommand not found: %s%s\n", state.GetColor(state.config.ErrorCol), command.args[0], state.Reset())
				return 127
			}
			process := exec.Command(path, command.args[1:]...)
			process.Stdin = input
			process.Stdout = &stageOutput
			process.Stderr = os.Stderr
			process.Env = commandEnvironment(state)
			if err := process.Run(); err != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
				stageStatus = processExitStatus(err)
			}
		}
		if stageStatus != 0 {
			if _, err := io.Copy(output, &stageOutput); err != nil {
				return 1
			}
			return stageStatus
		}

		if command.stdoutPath != "" {
			flags := os.O_CREATE | os.O_WRONLY
			if command.appendOut {
				flags |= os.O_APPEND
			} else {
				flags |= os.O_TRUNC
			}
			file, err := os.OpenFile(command.stdoutPath, flags, 0644)
			if err != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
				return 1
			}
			_, writeErr := file.Write(stageOutput.Bytes())
			file.Close()
			if writeErr != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), writeErr, state.Reset())
				return 1
			}
			stageOutput.Reset()
		}

		if i == len(commands)-1 {
			_, _ = io.Copy(output, &stageOutput)
		} else {
			input = bytes.NewReader(stageOutput.Bytes())
		}
	}
	return 0
}

func processExitStatus(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() >= 0 {
		return exitErr.ExitCode()
	}
	return 1
}

func runPipeline(state *State, commands []PipelineCommand, output io.Writer) int {
	if len(commands) == 0 {
		return 0
	}
	if len(commands) == 1 {
		command := commands[0]
		if !isPipelineInternal(command.args[0]) {
			path, err := exec.LookPath(command.args[0])
			if err != nil {
				fmt.Fprintf(output, "%sCommand not found: %s%s\n", state.GetColor(state.config.ErrorCol), command.args[0], state.Reset())
				return 127
			}
			if !authorizeExecutable(state, path, output) {
				return 126
			}
		}
		var stdin io.Reader = os.Stdin
		var inputFile *os.File
		if command.stdinPath != "" {
			if strings.HasPrefix(command.args[0], "!") || command.args[0] == "cd" || command.args[0] == "ls" {
				fmt.Fprintf(output, "%sinput redirection is not supported for internal commands%s\n", state.GetColor(state.config.ErrorCol), state.Reset())
				return 1
			}
			file, err := os.Open(command.stdinPath)
			if err != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
				return 1
			}
			inputFile = file
			defer inputFile.Close()
			stdin = inputFile
		}
		if command.stdoutPath == "" {
			return runWithStdinApproved(state, command.args, output, stdin)
		}
		flags := os.O_CREATE | os.O_WRONLY
		if command.appendOut {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		file, err := os.OpenFile(command.stdoutPath, flags, 0644)
		if err != nil {
			fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
			return 1
		}
		defer file.Close()
		return runWithStdinApproved(state, command.args, file, stdin)
	}

	for i, command := range commands {
		if len(command.args) == 0 {
			fmt.Fprintf(output, "%sempty command in pipeline%s\n", state.GetColor(state.config.ErrorCol), state.Reset())
			return 2
		}
		if !isPipelineInternal(command.args[0]) {
			path, err := exec.LookPath(command.args[0])
			if err != nil {
				fmt.Fprintf(output, "%sCommand not found: %s%s\n", state.GetColor(state.config.ErrorCol), command.args[0], state.Reset())
				return 127
			}
			if !authorizeExecutable(state, path, output) {
				return 126
			}
		}
		if isPipelineInternal(command.args[0]) ||
			(i < len(commands)-1 && command.stdoutPath != "") || (i > 0 && command.stdinPath != "") {
			return runBufferedPipeline(state, commands, output)
		}
	}

	execs := make([]*exec.Cmd, len(commands))
	readers := make([]io.ReadCloser, len(commands)-1)
	writers := make([]io.WriteCloser, len(commands)-1)
	var redirectedFiles []*os.File
	defer func() {
		for _, file := range redirectedFiles {
			_ = file.Close()
		}
		for _, reader := range readers {
			if reader != nil {
				_ = reader.Close()
			}
		}
		for _, writer := range writers {
			if writer != nil {
				_ = writer.Close()
			}
		}
	}()
	for i, command := range commands {
		path, err := exec.LookPath(command.args[0])
		if err != nil {
			fmt.Fprintf(output, "%sCommand not found: %s%s\n", state.GetColor(state.config.ErrorCol), command.args[0], state.Reset())
			return 127
		}
		execs[i] = exec.Command(path, command.args[1:]...)
		execs[i].Env = commandEnvironment(state)
		execs[i].Stderr = os.Stderr
		if i > 0 {
			execs[i].Stdin = readers[i-1]
		} else if command.stdinPath != "" {
			file, err := os.Open(command.stdinPath)
			if err != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
				return 1
			}
			execs[i].Stdin = file
			redirectedFiles = append(redirectedFiles, file)
		}
		if i < len(commands)-1 {
			reader, writer, err := os.Pipe()
			if err != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
				return 1
			}
			readers[i] = reader
			writers[i] = writer
			execs[i].Stdout = writer
		} else if command.stdoutPath != "" {
			flags := os.O_CREATE | os.O_WRONLY
			if command.appendOut {
				flags |= os.O_APPEND
			} else {
				flags |= os.O_TRUNC
			}
			file, err := os.OpenFile(command.stdoutPath, flags, 0644)
			if err != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
				return 1
			}
			execs[i].Stdout = file
			redirectedFiles = append(redirectedFiles, file)
		} else {
			execs[i].Stdout = output
		}
	}

	started := make([]bool, len(execs))
	for i, command := range execs {
		if err := command.Start(); err != nil {
			fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
			for _, writer := range writers {
				if writer != nil {
					_ = writer.Close()
				}
			}
			for _, reader := range readers {
				if reader != nil {
					_ = reader.Close()
				}
			}
			for j, startedCommand := range execs {
				if started[j] {
					_ = startedCommand.Wait()
				}
			}
			return 1
		}
		started[i] = true
	}
	for _, writer := range writers {
		writer.Close()
	}
	for _, reader := range readers {
		reader.Close()
	}
	status := 0
	for i, command := range execs {
		err := command.Wait()
		if i == len(execs)-1 {
			status = processExitStatus(err)
			if err != nil {
				fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
			}
		}
	}
	return status
}

func RunString(state *State, input string) {
	RunStringFromSource(state, input, SourceInteractive)
}

func RunStringFromSource(state *State, input string, source CommandSource) {
	RunStringToSource(state, input, os.Stdout, source)
}

func RunStringTo(state *State, input string, output io.Writer) {
	RunStringToSource(state, input, output, SourceInteractive)
}

func RunStringToSource(state *State, input string, output io.Writer, source CommandSource) {
	previousSource := SourceInteractive
	if state != nil {
		previousSource = state.commandSource
		state.commandSource = source
		defer func() {
			state.commandSource = previousSource
		}()
	}

	tokens := Lex(input)
	tokenSlice := []Token{}
	tokens.ForEach(func(idx int, token Token) {
		tokenSlice = append(tokenSlice, token)
	})

	commands, err := parsePipeline(state, tokenSlice)
	if err != nil {
		fmt.Fprintf(os.Stdout, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
		if state != nil {
			state.lastExitCode = 2
		}
		return
	}
	if len(commands) == 0 {
		return
	}
	status := runPipeline(state, commands, output)
	if state != nil {
		state.lastExitCode = status
	}
}

func Run(state *State, cmdArgs []string, w io.Writer) int {
	return runWithStdin(state, cmdArgs, w, os.Stdin)
}

func runWithStdin(state *State, cmdArgs []string, w io.Writer, stdin io.Reader) int {
	return runWithStdinPolicy(state, cmdArgs, w, stdin, true)
}

func runWithStdinApproved(state *State, cmdArgs []string, w io.Writer, stdin io.Reader) int {
	return runWithStdinPolicy(state, cmdArgs, w, stdin, false)
}

func runWithStdinPolicy(state *State, cmdArgs []string, w io.Writer, stdin io.Reader, checkTrust bool) int {
	if len(cmdArgs) == 0 {
		return 0
	}

	if !strings.HasPrefix(cmdArgs[0], "!") {
		if cmdArgs[0] == "cd" {
			if len(cmdArgs) > 1 {
				target := UnformatPathIfInHome(cmdArgs[1])
				err := os.Chdir(target)
				if err != nil {
					fmt.Fprintf(w, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
					return 1
				}
				state.workspaces.Get(state.cur_workspace).IfPresent(func(ws *Workspace) {
					ws.path, _ = os.Getwd()
				})
			}
			return 0
		}

		if cmdArgs[0] == "ls" {
			return state.PrettyLS(w, cmdArgs)
		}

		if is_windows {
			if RunWinCommands(cmdArgs, w) {
				return 0
			}
		}

		cmdPath, err := exec.LookPath(cmdArgs[0])
		if err != nil {
			fmt.Fprintf(w, "%sCommand not found: %s%s\n", state.GetColor(state.config.ErrorCol), cmdArgs[0], state.Reset())
			return 127
		}

		if checkTrust && !authorizeExecutable(state, cmdPath, w) {
			return 126
		}
		c := exec.Command(cmdPath, cmdArgs[1:]...)
		c.Stdout = w
		c.Stdin = stdin
		c.Stderr = os.Stderr

		envMap := make(map[string]string)
		for _, e := range os.Environ() {
			pair := strings.SplitN(e, "=", 2)
			if len(pair) == 2 {
				envMap[pair[0]] = pair[1]
			}
		}

		if state != nil {
			state.workspaces.Get(state.cur_workspace).IfPresent(func(ws *Workspace) {
				ws.scopes.ForEach(func(idx int, s *Scope) {
					s.overrides.ForEach(func(key string, val string) {
						envMap[key] = val
					})
				})
			})
		}

		finalEnv := make([]string, 0, len(envMap))
		for k, v := range envMap {
			finalEnv = append(finalEnv, fmt.Sprintf("%s=%s", k, v))
		}
		c.Env = finalEnv

		err = c.Run()
		if err != nil {
			fmt.Fprintf(w, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
		}
		return processExitStatus(err)
	} else {
		res := HandleStateCommands(state, cmdArgs)
		status := 0
		res.IfPresent(func(err error) {
			fmt.Fprintf(w, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
			status = 1
		})
		return status
	}
	return 0
}

func ReadConfiguration(state *State) {
	configFilePath := state.configPath
	isDefaultConfig := configFilePath == ""
	if isDefaultConfig {
		path, err := os.UserConfigDir()
		if err != nil {
			fmt.Printf("Could not find User config directory.\n")
			return
		}

		path = filepath.Join(path, ".loin")
		err = os.MkdirAll(path, 0700)
		if err != nil {
			fmt.Printf("Error creating configuration directory: %v\n", err)
			return
		}

		configFilePath = filepath.Join(path, "default.cloth")
		if err := createDefaultClothIfMissing(configFilePath); err != nil {
			fmt.Printf("Error ensuring configuration file exists: %v\n", err)
			return
		}
	}

	source := configurationCommandSource(isDefaultConfig, false)
	var data []byte
	var err error
	if isDefaultConfig {
		data, err = readProtectedDefaultCloth(configFilePath)
		if err == nil {
			source = configurationCommandSource(true, true)
		} else {
			fmt.Printf("Warning: default.cloth is not trusted (%v); treating its commands as gray-listed.\n", err)
			data, err = readDefaultCloth(configFilePath)
		}
	} else {
		data, err = os.ReadFile(configFilePath)
	}
	state.configSource = source
	if err != nil {
		fmt.Printf("Error reading configuration: %v\n", err)
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			RunStringFromSource(state, line, source)
		}
	}
	if source == SourceDefaultCloth {
		warnMissingDefaultClothTrustTargets(os.Stderr)
	}
}

func createDefaultClothIfMissing(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create default.cloth: %w", err)
	}

	content := strings.Join(generatedDefaultClothCommands(), "\n") + "\n"
	_, writeErr := io.WriteString(file, content)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		removeErr := os.Remove(path)
		return errors.Join(writeErr, closeErr, removeErr)
	}
	return nil
}

func validateSelectedCloth(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("configuration file %q does not exist", path)
		}
		return fmt.Errorf("configuration file %q is unreadable: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("configuration path %q is a directory, not a file", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("configuration path %q is not a regular file", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("configuration file %q is unreadable: %w", path, err)
	}
	return f.Close()
}

func selectedClothWarning(path string) string {
	defaultDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Sprintf("Warning: selected configuration %q is outside the protected default configuration location.", path)
	}
	defaultPath := filepath.Join(defaultDir, ".loin", "default.cloth")
	selectedAbs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Sprintf("Warning: selected configuration %q is outside the protected default configuration location.", path)
	}
	defaultAbs, err := filepath.Abs(defaultPath)
	if err != nil || filepath.Clean(selectedAbs) != filepath.Clean(defaultAbs) {
		return fmt.Sprintf("Warning: selected configuration %q is outside the protected default configuration location.", path)
	}
	return ""
}

func RunNonInteractive(state *State) {
	if err := runNonInteractive(state, os.Stdin); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading command input: %v\n", err)
	}
}

func runNonInteractive(state *State, input io.Reader) error {
	scanner := bufio.NewScanner(input)
	var command strings.Builder

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		continued := strings.HasSuffix(line, "\\")
		if continued {
			line = strings.TrimSuffix(line, "\\")
		}
		command.WriteString(line)

		if continued {
			command.WriteByte(' ')
			continue
		}

		input := strings.TrimSpace(command.String())
		command.Reset()
		if input == "exit" {
			return nil
		}
		if input != "" {
			RunStringFromSource(state, input, SourceNonInteractive)
		}
	}
	return scanner.Err()
}

func main() {
	selectedCloth := flag.String("cloth", "", "load a specific .cloth configuration file")
	flag.Parse()

	InitTerminal()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	state := &State{
		cur_workspace: 0,
		workspaces:    ungo.NewLinkedList[*Workspace](),
		config:        DefaultConfiguration(),
	}

	start_dir, _ := os.Getwd()
	state.workspaces.Add(&Workspace{
		name:   "",
		path:   start_dir,
		scopes: ungo.NewLinkedList[*Scope](),
	})

	if info, err := os.Stdin.Stat(); err == nil {
		state.interactiveInput = info.Mode()&os.ModeCharDevice != 0
	}
	if *selectedCloth != "" {
		if err := validateSelectedCloth(*selectedCloth); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		if warning := selectedClothWarning(*selectedCloth); warning != "" {
			fmt.Fprintln(os.Stderr, warning)
		}
	}

	state.configPath = *selectedCloth
	state.ResetConfig()
	if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice == 0 {
		RunNonInteractive(state)
		return
	}

	duration := ungo.None[time.Duration]()

	for {
		input := Prompt(state, duration)
		fmt.Print(state.Reset())
		duration = ungo.None[time.Duration]()

		if input == "exit" {
			break
		}

		if input == "" {
			continue
		}

		start := time.Now()
		RunString(state, input)
		duration = ungo.Some(time.Since(start))
	}
}
