package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/loeredami/ungo"
)

var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"
var Yellow = "\033[33m"
var Blue = "\033[34m"
var Magenta = "\033[35m"
var Cyan = "\033[36m"
var Gray = "\033[37m"
var White = "\033[97m"

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

type Scope struct {
	name      string
	overrides *ungo.SmallMap[string, string]
}

type Workspace struct {
	path   string
	scopes *ungo.LinkedList[*Scope]
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
}

func (state *State) RefreshLine(prompt string, buffer []rune, cursor int) {
	fmt.Printf("\r\033[K%s%s", prompt, string(buffer))
	if cursor < len(buffer) {
		moveBack := len(buffer) - cursor
		fmt.Printf("\033[%dD", moveBack)
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
		style := White
		indicator := ""

		if entry.IsDir() {
			style = Blue
			indicator = "/"
		} else if entry.Type()&os.ModeSymlink != 0 {
			style = Cyan
			indicator = "@"
		} else if info, err := entry.Info(); err == nil && info.Mode()&0111 != 0 {
			style = Green
			indicator = "*"
		}

		fmt.Fprintf(w, "%s%-20s%s", style, name+indicator, Reset)

		if (i+1)%4 == 0 {
			fmt.Fprintln(w)
		}
	}
	fmt.Fprint(w, "\n\n")
}

func renderPromptInfo(state *State, time_taken ungo.Optional[time.Duration]) string {
	cur_dir, _ := os.Getwd()
	admin := os.Getuid() == 0
	in_sign := ungo.If(admin, fmt.Sprintf("%s#SUDO»%s", Red, Reset), fmt.Sprintf("%s»%s", Magenta, Reset))

	state.workspaces.ForEach(func(idx int, ws *Workspace) {
		if state.workspaces.Size() != 1 {
			fmt.Printf("[%s%d%s] ", Cyan, idx, Reset)
		}
		fmt.Printf("%s%s%s", Blue, ReformatPathIfInHome(ws.path), Reset)

		if state.workspaces.Size() != 1 {
			if idx == state.cur_workspace {
				fmt.Printf(" [%s*%s]", Cyan, Reset)
			} else if cur_dir == ws.path {
				fmt.Printf(" [%sH%s]", Yellow, Reset)
			}
		}

		if idx == state.cur_workspace {
			if _, err := os.ReadDir(".git"); err == nil {
				if header, err := os.ReadFile("./.git/HEAD"); err == nil {
					splitted := strings.Split(string(header), "/")
					branch_name := strings.TrimSpace(splitted[len(splitted)-1])
					fmt.Printf(" (%s%s%s)", Green, branch_name, Reset)
				}
			}
		}

		time_taken.IfPresent(func(duration time.Duration) {
			if idx == state.cur_workspace {
				fmt.Printf(" %s~%s%v%s", Cyan, Yellow, duration, Reset)
			}
		})
		fmt.Print("\n")
	})

	state.workspaces.Get(state.cur_workspace).IfPresent(func(w *Workspace) {
		w.scopes.ForEach(func(idx int, s *Scope) {
			fmt.Printf(":%s%s%s", Yellow, s.name, Reset)
		})
	})

	promptStr := fmt.Sprintf("%s%s ", in_sign, Cyan)
	fmt.Print(promptStr)
	return promptStr
}

func readRawInput(state *State, promptStr string) string {
	var buffer []rune
	cursor := 0
	historyIdx := len(state.history)

	b := make([]byte, 1024)
	for {
		n, _ := os.Stdin.Read(b)
		if n == 0 {
			continue
		}

		needsRefresh := false

		for i := 0; i < n; i++ {
			char := b[i]

			if char != 9 {
				state.lastWasTab = false
			}

			switch char {
			case 13, 10: // Enter
				input := strings.TrimSpace(string(buffer))
				if input != "" && (len(state.history) == 0 || state.history[len(state.history)-1] != input) {
					state.history = append(state.history, input)
				}
				fmt.Print("\r\n")
				return input

			case 127, 8: // Backspace
				if cursor > 0 {
					buffer = append(buffer[:cursor-1], buffer[cursor:]...)
					cursor--
					needsRefresh = true
				}

			case 9: // Tab
				buffer = state.HandleAutocomplete(buffer, &cursor)
				needsRefresh = true
				state.lastWasTab = true

			case 27: // Arrows
				if i+2 < n && b[i+1] == 91 {
					switch b[i+2] {
					case 65: // Up
						if historyIdx > 0 {
							historyIdx--
							buffer = []rune(state.history[historyIdx])
							cursor = len(buffer)
							needsRefresh = true
						}
					case 66: // Down
						if historyIdx < len(state.history)-1 {
							historyIdx++
							buffer = []rune(state.history[historyIdx])
							cursor = len(buffer)
							needsRefresh = true
						} else if historyIdx == len(state.history)-1 {
							historyIdx = len(state.history)
							buffer = []rune("")
							cursor = 0
							needsRefresh = true
						}
					case 68: // Left
						if cursor > 0 {
							cursor--
							needsRefresh = true
						}
					case 67: // Right
						if cursor < len(buffer) {
							cursor++
							needsRefresh = true
						}
					}
					i += 2
				}

			case 3: // Ctrl+C
				fmt.Print("^C\r\n")
				return ""

			case 4: // Ctrl+D
				return "exit"

			default:
				if char >= 32 && char <= 126 {
					charRune := rune(char)
					buffer = append(buffer[:cursor], append([]rune{charRune}, buffer[cursor:]...)...)
					cursor++
					needsRefresh = true
				}
			}
		}

		if needsRefresh {
			state.RefreshLine(promptStr, buffer, cursor)
		}
	}
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
			if token.Type == Path || token.Type == String {
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
			fmt.Fprintf(w, "%s%v%s\n", Red, err, Reset)
		}
	} else {
		res := HandleStateCommands(state, cmdArgs)
		res.IfPresent(func(err error) {
			fmt.Fprintf(w, "%s%v%s\n", Red, err, Reset)
		})
	}
}

func main() {
	InitTerminal()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	state := &State{
		cur_workspace: 0,
		workspaces:    ungo.NewLinkedList[*Workspace](),
	}

	start_dir, _ := os.Getwd()
	state.workspaces.Add(&Workspace{
		path:   start_dir,
		scopes: ungo.NewLinkedList[*Scope](),
	})

	duration := ungo.None[time.Duration]()

	for {
		input := Prompt(state, duration)
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
