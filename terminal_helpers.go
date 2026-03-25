package main

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/loeredami/ungo"
)

func (state *State) findWordBoundaryLeft(buffer []rune, cursor int) int {
	pos := cursor
	for pos > 0 && unicode.IsSpace(buffer[pos-1]) {
		pos--
	}
	for pos > 0 && !unicode.IsSpace(buffer[pos-1]) {
		pos--
	}
	return pos
}

func (state *State) findWordBoundaryRight(buffer []rune, cursor int) int {
	pos := cursor
	length := len(buffer)
	for pos < length && unicode.IsSpace(buffer[pos]) {
		pos++
	}
	for pos < length && !unicode.IsSpace(buffer[pos]) {
		pos++
	}
	return pos
}

func readRawInput(state *State, promptStr string) string {
	var buffer []rune
	cursor := 0
	historyIdx := len(state.history)

	b := make([]byte, 1024)
	for {
		n, err := os.Stdin.Read(b)
		if n == 0 || err != nil {
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

			case 23, 31: // Ctrl+W or Ctrl+Backspace
				if cursor > 0 {
					newPos := state.findWordBoundaryLeft(buffer, cursor)
					buffer = append(buffer[:newPos], buffer[cursor:]...)
					cursor = newPos
					needsRefresh = true
				}

			case 9: // Tab
				buffer = state.HandleAutocomplete(buffer, &cursor)
				needsRefresh = true
				state.lastWasTab = true

			case 3: // Ctrl+C
				fmt.Print("^C\r\n")
				return ""

			case 4: // Ctrl+D
				return "exit"

			case 27: // Escape sequences
				if i+1 < n && (b[i+1] == 'd' || b[i+1] == 'D') {
					newEnd := state.findWordBoundaryRight(buffer, cursor)
					buffer = append(buffer[:cursor], buffer[newEnd:]...)
					i++
					needsRefresh = true
				} else if i+1 < n && (b[i+1] == '\b' || b[i+1] == 127) {
					if cursor > 0 {
						newPos := state.findWordBoundaryLeft(buffer, cursor)
						buffer = append(buffer[:newPos], buffer[cursor:]...)
						cursor = newPos
						needsRefresh = true
					}
					i++
				} else if i+2 < n && b[i+1] == '[' {
					isCtrl := false
					if i+4 < n && b[i+3] == ';' && b[i+4] == '5' {
						isCtrl = true
					}

					switch b[i+2] {
					case 'A': // Up
						if historyIdx > 0 {
							historyIdx--
							buffer = []rune(state.history[historyIdx])
							cursor = len(buffer)
							needsRefresh = true
						}
						i += 2
					case 'B': // Down
						if historyIdx < len(state.history)-1 {
							historyIdx++
							buffer = []rune(state.history[historyIdx])
							cursor = len(buffer)
						} else if historyIdx == len(state.history)-1 {
							historyIdx = len(state.history)
							buffer = []rune("")
							cursor = 0
						}
						needsRefresh = true
						i += 2
					case 'D': // Left
						if isCtrl && i+5 < n {
							cursor = state.findWordBoundaryLeft(buffer, cursor)
							i += 5
						} else {
							if cursor > 0 {
								cursor--
							}
							i += 2
						}
						needsRefresh = true
					case 'C': // Right
						if isCtrl && i+5 < n {
							cursor = state.findWordBoundaryRight(buffer, cursor)
							i += 5
						} else {
							if cursor < len(buffer) {
								cursor++
							}
							i += 2
						}
						needsRefresh = true
					case '5': // Ctrl + Arrows Alternate (\033[5C, \033[5D)
						if i+3 < n {
							switch b[i+3] {
							case 'C':
								cursor = state.findWordBoundaryRight(buffer, cursor)
							case 'D':
								cursor = state.findWordBoundaryLeft(buffer, cursor)
							}
							i += 3
							needsRefresh = true
						} else {
							for j := i + 1; j < n; j++ {
								if (b[j] >= 'A' && b[j] <= 'Z') || (b[j] >= 'a' && b[j] <= 'z') || b[j] == '~' {
									i = j
									break
								}
							}
						}
					case '3': // Delete Variants
						if isCtrl && i+5 < n && (b[i+5] == '~' || b[i+5] == 'D' || b[i+5] == 'd') {
							newEnd := state.findWordBoundaryRight(buffer, cursor)
							buffer = append(buffer[:cursor], buffer[newEnd:]...)
							i += 5
						} else if i+3 < n && b[i+3] == '~' {
							if cursor < len(buffer) {
								buffer = append(buffer[:cursor], buffer[cursor+1:]...)
							}
							i += 3
						} else {
							for j := i + 1; j < n; j++ {
								if (b[j] >= 'A' && b[j] <= 'Z') || (b[j] >= 'a' && b[j] <= 'z') || b[j] == '~' {
									i = j
									break
								}
							}
						}
						needsRefresh = true
					case '1': // Home / Modified Arrows
						if isCtrl && i+5 < n {
							switch b[i+5] {
							case 'C':
								cursor = state.findWordBoundaryRight(buffer, cursor)
							case 'D':
								cursor = state.findWordBoundaryLeft(buffer, cursor)
							}
							i += 5
							needsRefresh = true
						} else {
							for j := i + 1; j < n; j++ {
								if (b[j] >= 'A' && b[j] <= 'Z') || (b[j] >= 'a' && b[j] <= 'z') || b[j] == '~' {
									i = j
									break
								}
							}
						}
					default:
						for j := i + 1; j < n; j++ {
							if (b[j] >= 'A' && b[j] <= 'Z') || (b[j] >= 'a' && b[j] <= 'z') || b[j] == '~' {
								i = j
								break
							}
						}
					}
				}

			default:
				if char >= 32 && char <= 126 {
					charRune := rune(char)

					if charRune == '{' {
						shouldAddClosing := cursor == len(buffer) || buffer[cursor] != '}'

						if shouldAddClosing {
							buffer = append(buffer[:cursor], append([]rune{'{', '}'}, buffer[cursor:]...)...)
							cursor++
							needsRefresh = true
							break
						}
					}

					// Standard insertion for all other characters
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

func renderPromptInfo(state *State, time_taken ungo.Optional[time.Duration]) string {
	cur_dir, _ := os.Getwd()
	admin := os.Getuid() == 0
	in_sign := ungo.If(admin, fmt.Sprintf("%s%s%s", state.GetColor(state.config.SudoPromptCol), state.config.SudoPrompt, state.Reset()), fmt.Sprintf("%s%s%s", state.GetColor(state.config.PromptCol), state.config.Prompt, state.Reset()))

	state.workspaces.ForEach(func(idx int, ws *Workspace) {
		if state.workspaces.Size() != 1 {
			fmt.Printf("[%s%d%s] ", state.GetColor(state.config.IdxCol), idx, state.Reset())
		}
		fmt.Printf("%s%s%s", state.GetColor(state.config.PathCol), ReformatPathIfInHome(ws.path), state.Reset())

		if state.workspaces.Size() != 1 {
			if idx == state.cur_workspace {
				fmt.Printf(" [%s*%s]", state.GetColor(state.config.CurWSCol), state.Reset())
			} else if cur_dir == ws.path {
				fmt.Printf(" [%sH%s]", state.GetColor(state.config.CurDirCol), state.Reset())
			}
		}

		if idx == state.cur_workspace {
			if _, err := os.ReadDir(".git"); err == nil {
				if header, err := os.ReadFile("./.git/HEAD"); err == nil {
					splitted := strings.Split(string(header), "/")
					branch_name := strings.TrimSpace(splitted[len(splitted)-1])
					fmt.Printf(" (%s%s%s)", state.GetColor(state.config.GitBranchCol), branch_name, state.Reset())
				}
			}
		}

		time_taken.IfPresent(func(duration time.Duration) {
			if idx == state.cur_workspace {
				fmt.Printf(" %s~%s%v%s", state.GetColor(state.config.TimePrefixCol), state.GetColor(state.config.TimeCol), duration, state.Reset())
			}
		})
		fmt.Print("\n")
	})

	state.workspaces.Get(state.cur_workspace).IfPresent(func(w *Workspace) {
		w.scopes.ForEach(func(idx int, s *Scope) {
			fmt.Printf("%s%s%s%s", state.config.ScopeSign, state.GetColor(state.config.ScopeCol), s.name, state.Reset())
		})
	})

	promptStr := fmt.Sprintf("%s%s ", in_sign, state.GetColor(state.config.InputCol))
	fmt.Print(promptStr)
	return promptStr
}

func (state *State) highlightInput(buffer []rune) string {
	if !state.config.ColorMode || len(buffer) == 0 {
		return string(buffer)
	}

	raw := string(buffer)
	tokens := Lex(raw)
	var highlighted strings.Builder

	currentIdx := 0

	tokens.ForEach(func(idx int, t Token) {
		if t.Type == EndOfInput {
			return
		}

		t.Value.IfPresent(func(val string) {
			foundIdx := strings.Index(raw[currentIdx:], val)

			if foundIdx != -1 {
				foundIdx += currentIdx

				if foundIdx > currentIdx {
					highlighted.WriteString(state.Reset() + raw[currentIdx:foundIdx] + state.Reset())
				}

				color := state.Reset()
				switch t.Type {
				case Identifier:
					color = state.GetColor(state.config.InputCol)
				case String:
					color = state.GetColor(state.config.InputStringCol)
				case Number:
					color = state.GetColor(state.config.InputNumCol)
				case Path:
					color = state.GetColor(state.config.InputPathCol)
				case Varname:
					color = state.GetColor(state.config.InputVarCol)
				case OpenBrace, CloseBrace:
					color = state.GetColor(state.config.InputBraceCol)
				}

				highlighted.WriteString(color + val + state.Reset())
				currentIdx = foundIdx + len(val)
			}
		})
	})

	if currentIdx < len(raw) {
		highlighted.WriteString(state.Reset() + raw[currentIdx:] + state.Reset())
	}

	return highlighted.String()
}
