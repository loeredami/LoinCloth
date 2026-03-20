package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	historyIndex  int // Track current position in history while navigating
}

func (state *State) RefreshLine(prompt string, buffer []rune, cursor int) {
	fmt.Printf("\r\033[K%s%s", prompt, string(buffer))
	if cursor < len(buffer) {
		moveBack := len(buffer) - cursor
		fmt.Printf("\033[%dD", moveBack)
	}
}

func (state *State) HandleAutocomplete(buffer []rune, cursor *int) []rune {
	currentLine := string(buffer[:*cursor])

	var lastArgRaw strings.Builder
	var lastArgUnescaped strings.Builder
	inQuotes := false
	escaped := false

	for _, r := range currentLine {
		if escaped {
			lastArgRaw.WriteRune(r)
			lastArgUnescaped.WriteRune(r)
			escaped = false
			continue
		}

		if r == '\\' {
			lastArgRaw.WriteRune(r)
			escaped = true
			continue
		}

		if r == '"' {
			lastArgRaw.WriteRune(r)
			inQuotes = !inQuotes
			continue
		}

		if r == ' ' && !inQuotes {
			lastArgRaw.Reset()
			lastArgUnescaped.Reset()
			continue
		}

		lastArgRaw.WriteRune(r)
		lastArgUnescaped.WriteRune(r)
	}

	searchPath := lastArgUnescaped.String()
	if strings.HasPrefix(searchPath, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			if len(searchPath) == 1 {
				searchPath = home
			} else if searchPath[1] == '/' {
				searchPath = home + searchPath[1:]
			}
		}
	}

	dir := "."
	prefix := searchPath
	lastSlash := strings.LastIndex(searchPath, "/")
	if lastSlash != -1 {
		if lastSlash == 0 {
			dir = "/"
		} else {
			dir = searchPath[:lastSlash]
		}
		prefix = searchPath[lastSlash+1:]
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

			newSuffix := []rune(appendStr.String())
			buffer = append(buffer[:*cursor], append(newSuffix, buffer[*cursor:]...)...)
			*cursor += len(newSuffix)
			break
		}
	}

	return buffer
}

func (state *State) PrettyLS(cmdArgs []string) {
	path := "."
	if len(cmdArgs) > 1 {
		path = cmdArgs[1]
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		fmt.Printf("%sError: %v%s\n", Red, err, Reset)
		return
	}

	fmt.Println()
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

		fmt.Printf("%s%-20s%s", style, name+indicator, Reset)

		if (i+1)%4 == 0 {
			fmt.Println()
		}
	}
	fmt.Print("\n\n")
}

func Prompt(state *State, time_taken ungo.Optional[time.Duration]) string {
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
			entries, err := os.ReadDir(".git")
			if err == nil {
				for _, entry := range entries {
					if entry.Name() == "HEAD" {
						header, err := os.ReadFile("./.git/HEAD")
						if err == nil {
							splitted := strings.Split(string(header), "/")
							branch_name := strings.TrimSpace(splitted[len(splitted)-1])
							fmt.Printf(" (%s%s%s)", Green, branch_name, Reset)
							break
						}
					}
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

	fd := os.Stdin.Fd()
	oldState, err := MakeRaw(fd)
	if err != nil {
		var input string
		fmt.Scanln(&input)
		return input
	}
	defer RestoreTerminal(fd, oldState)

	var buffer []rune
	cursor := 0
	// Initialize history index to the end of history
	historyIdx := len(state.history)

	for {
		b := make([]byte, 3)
		n, _ := os.Stdin.Read(b)
		if n == 0 {
			continue
		}

		char := b[0]

		switch char {
		case 13, 10: // Enter
			input := strings.TrimSpace(string(buffer))
			if input != "" {
				// Avoid duplicate consecutive history entries
				if len(state.history) == 0 || state.history[len(state.history)-1] != input {
					state.history = append(state.history, input)
				}
			}
			fmt.Print("\r\n")
			return input

		case 127, 8: // Backspace
			if cursor > 0 {
				buffer = append(buffer[:cursor-1], buffer[cursor:]...)
				cursor--
				state.RefreshLine(promptStr, buffer, cursor)
			}

		case 9: // Tab (Autocomplete)
			buffer = state.HandleAutocomplete(buffer, &cursor)
			state.RefreshLine(promptStr, buffer, cursor)

		case 27: // Escape sequences
			if n > 2 && b[1] == 91 {
				switch b[2] {
				case 65: // Up Arrow (History Back)
					if historyIdx > 0 {
						historyIdx--
						buffer = []rune(state.history[historyIdx])
						cursor = len(buffer)
						state.RefreshLine(promptStr, buffer, cursor)
					}
				case 66: // Down Arrow (History Forward)
					if historyIdx < len(state.history)-1 {
						historyIdx++
						buffer = []rune(state.history[historyIdx])
						cursor = len(buffer)
						state.RefreshLine(promptStr, buffer, cursor)
					} else if historyIdx == len(state.history)-1 {
						historyIdx = len(state.history)
						buffer = []rune("")
						cursor = 0
						state.RefreshLine(promptStr, buffer, cursor)
					}
				case 68: // Left Arrow
					if cursor > 0 {
						cursor--
						state.RefreshLine(promptStr, buffer, cursor)
					}
				case 67: // Right Arrow
					if cursor < len(buffer) {
						cursor++
						state.RefreshLine(promptStr, buffer, cursor)
					}
				case 51: // Delete Key
					nextByte := make([]byte, 1)
					os.Stdin.Read(nextByte)
					if nextByte[0] == 126 {
						if cursor < len(buffer) {
							buffer = append(buffer[:cursor], buffer[cursor+1:]...)
							state.RefreshLine(promptStr, buffer, cursor)
						}
					}
				}
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
				state.RefreshLine(promptStr, buffer, cursor)
			}
		}
	}
}
func RunString(state *State, input string) {
	tokens := Lex(input)
	cmd := []string{}

	tokens.ForEach(func(idx int, token Token) {
		if token.Type == EndOfInput {
			return
		}

		token.Value.IfPresent(func(val string) {
			if token.Type == Path || token.Type == String {
				val = UnformatPathIfInHome(val)
			}

			if token.Type != Path && token.Type != String {
				env_val_opt := GetEnvValue(state, val)
				if env_val_opt.HasValue() {
					env_val := env_val_opt.Value()
					for _, v := range env_val {
						cmd = append(cmd, v)
					}
					return
				}
			}

			cmd = append(cmd, val)
		})
	})
	Run(state, cmd)
}

func Run(state *State, cmdArgs []string) {
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
			state.PrettyLS(cmdArgs)

			return
		}

		c := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		c.Stdout = os.Stdout
		c.Stdin = os.Stdin
		c.Stderr = os.Stderr

		err := c.Run()
		if err != nil {
			fmt.Printf("%s%v%s\n", Red, err, Reset)
		}
	} else {
		res := HandleStateCommands(state, cmdArgs)
		res.IfPresent(func(err error) {
			fmt.Printf("%s%v%s\n", Red, err, Reset)
		})
	}
}

func main() {
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
