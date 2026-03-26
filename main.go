package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

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

func (state *State) RefreshLine(prompt string, buffer []rune, cursor int) {
	termWidth, err := terminal.Width()
	if termWidth <= 0 || err != nil {
		termWidth = 80
	}

	highlightedBuffer := state.highlightInput(buffer)

	totalLen := uint(len(prompt) + len(buffer))
	rowCount := (totalLen + (termWidth) - 1) / termWidth

	fmt.Print("\r")

	if rowCount > 1 {
		fmt.Printf("\033[%dA", rowCount-1)
	}

	fmt.Print("\033[J")

	fmt.Printf("%s%s", prompt, highlightedBuffer)

	if cursor < len(buffer) {
		targetPos := len(prompt) + cursor
		currentPos := len(prompt) + len(buffer)

		moveBack := currentPos - targetPos
		if moveBack > 0 {
			fmt.Printf("\033[%dD", moveBack)
		}
	}
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
		var lastArgUnescaped strings.Builder
		inQuotes := false
		escaped := false

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

		dir := "."
		prefix := searchPath
		lastSlash := strings.LastIndex(resolvedSearchPath, "/")

		if lastSlash != -1 {
			if lastSlash == 0 {
				dir = "/"
			} else {
				dir = resolvedSearchPath[:lastSlash]
			}
			lastSlashInOriginal := strings.LastIndex(searchPath, "/")
			prefix = searchPath[lastSlashInOriginal+1:]
		} else if strings.HasPrefix(searchPath, "~") {
			dir = UnformatPathIfInHome("~")
			prefix = ""
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return buffer
		}

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

	if len(state.autoCompleteMatches) > 0 {
		match := state.autoCompleteMatches[state.autoCompleteIndex]
		newSuffix := []rune(match)
		buffer = append(buffer[:*cursor], append(newSuffix, buffer[*cursor:]...)...)
		*cursor += len(newSuffix)
		state.lastAddedLen = len(newSuffix)
	}

	return buffer
}

func (state *State) PrettyLS(w io.Writer, cmdArgs []string) {
	path := "."
	if len(cmdArgs) > 1 {
		path = cmdArgs[1]
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Fprintf(w, "%sError: %v%s\n", Red, err, Reset)
		return
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
			i = j

			innerArgs := processTokens(state, innerTokens)
			output := RunAndCapture(state, innerArgs)
			words := strings.Fields(output)
			cmd = append(cmd, words...)
			continue
		}

		token.Value.IfPresent(func(val string) {
			if token.Type == Path {
				val = UnformatPathIfInHome(val)
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

func RunString(state *State, input string) {
	tokens := Lex(input)
	var tokenSlice []Token
	tokens.ForEach(func(idx int, token Token) {
		tokenSlice = append(tokenSlice, token)
	})

	cmd := processTokens(state, tokenSlice)
	Run(state, cmd, os.Stdout)
}

func Run(state *State, cmdArgs []string, w io.Writer) {
	if len(cmdArgs) == 0 {
		return
	}

	if !strings.HasPrefix(cmdArgs[0], "!") {
		if cmdArgs[0] == "cd" {
			if len(cmdArgs) > 1 {
				os.Chdir(cmdArgs[1])
				state.workspaces.Get(state.cur_workspace).IfPresent(func(ws *Workspace) {
					ws.path, _ = os.Getwd()
				})
			}
			return
		}

		if cmdArgs[0] == "ls" {
			state.PrettyLS(w, cmdArgs)
			return
		}

		if is_windows {
			if RunWinCommands(cmdArgs) {
				return
			}
		}

		c := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		c.Stdout = w
		c.Stdin = os.Stdin
		c.Stderr = os.Stderr

		envMap := make(map[string]string)
		for _, e := range os.Environ() {
			pair := strings.SplitN(e, "=", 2)
			if len(pair) == 2 {
				envMap[pair[0]] = pair[1]
			}
		}

		state.workspaces.Get(state.cur_workspace).IfPresent(func(ws *Workspace) {
			ws.scopes.ForEach(func(idx int, s *Scope) {
				s.overrides.ForEach(func(key string, val string) {
					envMap[key] = val
				})
			})
		})

		finalEnv := make([]string, 0, len(envMap))
		for k, v := range envMap {
			finalEnv = append(finalEnv, fmt.Sprintf("%s=%s", k, v))
		}
		c.Env = finalEnv

		err := c.Run()
		if err != nil {
			fmt.Fprintf(w, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
		}
	} else {
		res := HandleStateCommands(state, cmdArgs)
		res.IfPresent(func(err error) {
			fmt.Fprintf(w, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
		})
	}
}

func ReadConfiguration(state *State) {
	path, err := os.UserConfigDir()
	if err != nil {
		fmt.Printf("Could not find User config directory.")
		return
	}

	path = filepath.Join(path, ".loin")

	os.Mkdir(path, os.ModePerm)

	f, _ := os.OpenFile(filepath.Join(path, "default.cloth"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	f.Close()

	data, err := os.ReadFile(filepath.Join(path, "default.cloth"))

	if err != nil {
		fmt.Printf("Error reading configuration: %v", err)
		return
	}

	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		RunString(state, line)
	}
}

func main() {
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
		path:   start_dir,
		scopes: ungo.NewLinkedList[*Scope](),
	})

	state.ResetConfig()

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
