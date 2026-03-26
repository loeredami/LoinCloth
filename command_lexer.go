package main

import (
	"strings"
	"unicode"

	"github.com/loeredami/ungo"
)

type TokenType int

const (
	Identifier TokenType = iota
	String
	Number
	Path
	EndOfInput
	Symbol
	Varname
	OpenBrace
	CloseBrace
)

type Token struct {
	Type  TokenType
	Value ungo.Optional[string]
}

type LexState struct {
	tokens *ungo.LinkedList[Token]
	input  string
}

func (ls LexState) IsDone() bool {
	return len(ls.input) == 0
}

func Lex(input string) *ungo.LinkedList[Token] {
	lexState := LexState{
		tokens: ungo.NewLinkedList[Token](),
		input:  input,
	}

	for !lexState.IsDone() {
		ungo.ShortPipe(&lexState, []func(*LexState) *LexState{
			// Skip Spaces
			func(state *LexState) *LexState {
				if state.IsDone() {
					return state
				}
				if unicode.IsSpace(rune(state.input[0])) {
					for !state.IsDone() && unicode.IsSpace(rune(state.input[0])) {
						state.input = state.input[1:]
					}
				}
				return state
			},

			// Braces
			func(state *LexState) *LexState {
				if state.IsDone() {
					return state
				}
				if state.input[0] == '{' {
					state.tokens.Add(Token{OpenBrace, ungo.Some("{")})
					state.input = state.input[1:]
				} else if state.input[0] == '}' {
					state.tokens.Add(Token{CloseBrace, ungo.Some("}")})
					state.input = state.input[1:]
				}
				return state
			},

			// Path
			func(state *LexState) *LexState {
				if state.IsDone() {
					return state
				}

				r := state.input[0]
				if r == '/' || r == '\\' || r == '~' || r == '.' || r == '*' {
					var builder strings.Builder
					for !state.IsDone() {
						curr := state.input[0]

						if curr == '\\' && len(state.input) > 1 && state.input[1] == ' ' {
							builder.WriteByte(' ') // Only write the space, consume the backslash
							state.input = state.input[2:]
							continue
						}

						if unicode.IsLetter(rune(curr)) || unicode.IsDigit(rune(curr)) ||
							strings.ContainsRune("/\\._-~@:+*", rune(curr)) {
							builder.WriteByte(curr)
							state.input = state.input[1:]
						} else {
							break
						}
					}
					state.tokens.Add(Token{
						Path,
						ungo.Some(builder.String()),
					})
				}
				return state
			},

			// Identifier
			func(state *LexState) *LexState {
				if state.IsDone() {
					return state
				}
				blacklist := "#${}*"
				if strings.Contains(blacklist, string([]byte{state.input[0]})) {
					return state
				}
				if !unicode.IsSpace(rune(state.input[0])) && !unicode.IsNumber(rune(state.input[0])) && state.input[0] != '"' {
					var builder strings.Builder
					for !state.IsDone() {
						curr := state.input[0]

						if curr == '\\' && len(state.input) > 1 && state.input[1] == ' ' {
							builder.WriteByte(' ')
							state.input = state.input[2:]
							continue
						}

						if unicode.IsSpace(rune(curr)) || curr == '{' || curr == '}' {
							break
						}

						builder.WriteByte(curr)
						state.input = state.input[1:]
					}
					state.tokens.Add(Token{Identifier, ungo.Some(builder.String())})
				}
				return state
			},

			// Number
			func(state *LexState) *LexState {
				if state.IsDone() {
					return state
				}
				if unicode.IsDigit(rune(state.input[0])) {
					tokenType := Number
					var builder strings.Builder
					for !state.IsDone() && (unicode.IsLetter(rune(state.input[0])) || unicode.IsDigit(rune(state.input[0]))) {
						if unicode.IsLetter(rune(state.input[0])) {
							tokenType = Identifier
						}
						builder.WriteByte(state.input[0])
						state.input = state.input[1:]
					}
					state.tokens.Add(Token{tokenType, ungo.Some(builder.String())})
				}
				return state
			},

			// String
			func(state *LexState) *LexState {
				if !state.IsDone() && state.input[0] == '"' {
					var builder strings.Builder
					state.input = state.input[1:]
					for !state.IsDone() && state.input[0] != '"' {
						builder.WriteByte(state.input[0])
						state.input = state.input[1:]
					}
					if !state.IsDone() {
						state.input = state.input[1:]
					}
					state.tokens.Add(Token{String, ungo.Some(builder.String())})
				}
				return state
			},

			// get var name
			func(state *LexState) *LexState {
				if !state.IsDone() && state.input[0] == '$' {
					state.input = state.input[1:]
					var builder strings.Builder

					for !state.IsDone() && !unicode.IsSpace(rune(state.input[0])) && state.input[0] != '}' {
						builder.WriteByte(state.input[0])
						state.input = state.input[1:]
					}

					state.tokens.Add(Token{Varname, ungo.Some(builder.String())})
				}

				return state
			},

			// skip comments
			func(state *LexState) *LexState {
				if state.IsDone() {
					return state
				}

				if len(state.input) >= 2 && state.input[0] == '#' && state.input[1] == '#' {
					state.input = state.input[2:] // Consume the ##

					for len(state.input) >= 2 {
						if state.input[0] == '#' && state.input[1] == '#' {
							state.input = state.input[2:] // Consume the closing ##
							return state
						}
						state.input = state.input[1:]
					}

					state.input = ""
					return state
				}

				if state.input[0] == '#' {
					state.tokens.Add(Token{Symbol, ungo.Some("#")})
					state.input = state.input[1:]
				}

				return state
			},
		})
	}

	lexState.tokens.Add(Token{EndOfInput, ungo.None[string]()})
	return lexState.tokens
}
