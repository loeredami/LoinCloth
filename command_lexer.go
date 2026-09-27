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
	Pipe
	RedirectOut
	RedirectAppend
	RedirectIn
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

func isEscapedShellCharacter(r byte) bool {
	return strings.ContainsRune("|><{}$#\\", rune(r))
}

var lexerPipeline = ungo.NewPipeSequence[*LexState](
	ungo.PipeFunc[*LexState](skipLexWhitespace),
	ungo.PipeFunc[*LexState](lexShellOperator),
	ungo.PipeFunc[*LexState](lexBrace),
	ungo.PipeFunc[*LexState](lexPath),
	ungo.PipeFunc[*LexState](lexIdentifier),
	ungo.PipeFunc[*LexState](lexNumber),
	ungo.PipeFunc[*LexState](lexString),
	ungo.PipeFunc[*LexState](lexVariable),
	ungo.PipeFunc[*LexState](lexComment),
)

func skipLexWhitespace(state *LexState) *LexState {
	if !state.IsDone() && unicode.IsSpace(rune(state.input[0])) {
		for !state.IsDone() && unicode.IsSpace(rune(state.input[0])) {
			state.input = state.input[1:]
		}
	}
	return state
}

func lexShellOperator(state *LexState) *LexState {
	if state.IsDone() {
		return state
	}
	switch state.input[0] {
	case '|':
		state.tokens.Add(Token{Pipe, ungo.Some("|")})
		state.input = state.input[1:]
	case '>':
		if len(state.input) > 1 && state.input[1] == '>' {
			state.tokens.Add(Token{RedirectAppend, ungo.Some(">>")})
			state.input = state.input[2:]
		} else {
			state.tokens.Add(Token{RedirectOut, ungo.Some(">")})
			state.input = state.input[1:]
		}
	case '<':
		state.tokens.Add(Token{RedirectIn, ungo.Some("<")})
		state.input = state.input[1:]
	}
	return state
}

func lexBrace(state *LexState) *LexState {
	if state.IsDone() {
		return state
	}
	switch state.input[0] {
	case '{':
		state.tokens.Add(Token{OpenBrace, ungo.Some("{")})
		state.input = state.input[1:]
	case '}':
		state.tokens.Add(Token{CloseBrace, ungo.Some("}")})
		state.input = state.input[1:]
	}
	return state
}

func lexPath(state *LexState) *LexState {
	if state.IsDone() {
		return state
	}
	r := state.input[0]
	if r == '\\' && len(state.input) > 1 && isEscapedShellCharacter(state.input[1]) {
		return state
	}
	if r != '/' && r != '\\' && r != '~' && r != '.' && r != '*' {
		return state
	}

	var builder strings.Builder
	for !state.IsDone() {
		curr := state.input[0]
		if curr == '\\' && len(state.input) > 1 && state.input[1] == ' ' {
			builder.WriteByte(' ')
			state.input = state.input[2:]
			continue
		}
		if curr == '\\' && len(state.input) > 1 && isEscapedShellCharacter(state.input[1]) {
			builder.WriteByte(state.input[1])
			state.input = state.input[2:]
			continue
		}
		if unicode.IsLetter(rune(curr)) || unicode.IsDigit(rune(curr)) ||
			strings.ContainsRune("/\\._-~@:+*", rune(curr)) {
			builder.WriteByte(curr)
			state.input = state.input[1:]
			continue
		}
		break
	}
	state.tokens.Add(Token{Path, ungo.Some(builder.String())})
	return state
}

func lexIdentifier(state *LexState) *LexState {
	if state.IsDone() || strings.ContainsRune("#${}*", rune(state.input[0])) {
		return state
	}
	if unicode.IsSpace(rune(state.input[0])) || unicode.IsNumber(rune(state.input[0])) || state.input[0] == '"' {
		return state
	}

	var builder strings.Builder
	for !state.IsDone() {
		curr := state.input[0]
		if curr == '\\' && len(state.input) > 1 && state.input[1] == ' ' {
			builder.WriteByte(' ')
			state.input = state.input[2:]
			continue
		}
		if curr == '\\' && len(state.input) > 1 && isEscapedShellCharacter(state.input[1]) {
			builder.WriteByte(state.input[1])
			state.input = state.input[2:]
			continue
		}
		if unicode.IsSpace(rune(curr)) || curr == '{' || curr == '}' || strings.ContainsRune("|><", rune(curr)) {
			break
		}
		builder.WriteByte(curr)
		state.input = state.input[1:]
	}
	state.tokens.Add(Token{Identifier, ungo.Some(builder.String())})
	return state
}

func lexNumber(state *LexState) *LexState {
	if state.IsDone() || !unicode.IsDigit(rune(state.input[0])) {
		return state
	}

	tokenType := Number
	var builder strings.Builder
	for !state.IsDone() && (unicode.IsLetter(rune(state.input[0])) || unicode.IsDigit(rune(state.input[0])) || state.input[0] == ';') {
		if unicode.IsLetter(rune(state.input[0])) {
			tokenType = Identifier
		}
		builder.WriteByte(state.input[0])
		state.input = state.input[1:]
	}
	state.tokens.Add(Token{tokenType, ungo.Some(builder.String())})
	return state
}

func lexString(state *LexState) *LexState {
	if state.IsDone() || state.input[0] != '"' {
		return state
	}

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
	return state
}

func lexVariable(state *LexState) *LexState {
	if state.IsDone() || state.input[0] != '$' {
		return state
	}

	state.input = state.input[1:]
	var builder strings.Builder
	for !state.IsDone() && !unicode.IsSpace(rune(state.input[0])) && state.input[0] != '}' {
		builder.WriteByte(state.input[0])
		state.input = state.input[1:]
	}
	state.tokens.Add(Token{Varname, ungo.Some(builder.String())})
	return state
}

func lexComment(state *LexState) *LexState {
	if state.IsDone() {
		return state
	}
	if len(state.input) >= 2 && state.input[0] == '#' && state.input[1] == '#' {
		state.input = state.input[2:]
		for len(state.input) >= 2 {
			if state.input[0] == '#' && state.input[1] == '#' {
				state.input = state.input[2:]
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
}

func Lex(input string) *ungo.LinkedList[Token] {
	lexState := LexState{
		tokens: ungo.NewLinkedList[Token](),
		input:  input,
	}
	for !lexState.IsDone() {
		lexerPipeline.Run(&lexState)
	}
	lexState.tokens.Add(Token{EndOfInput, ungo.None[string]()})
	return lexState.tokens
}
