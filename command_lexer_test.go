package main

import "testing"

func lexTokens(input string) []Token {
	tokens := []Token{}
	Lex(input).ForEach(func(_ int, token Token) {
		tokens = append(tokens, token)
	})
	return tokens
}

func TestLexShellOperators(t *testing.T) {
	tokens := lexTokens(`echo value|cat>output.txt>>append.txt<input.txt`)
	got := []TokenType{}
	for _, token := range tokens {
		got = append(got, token.Type)
	}

	want := []TokenType{
		Identifier,
		Identifier,
		Pipe,
		Identifier,
		RedirectOut,
		Identifier,
		RedirectAppend,
		Identifier,
		RedirectIn,
		Identifier,
		EndOfInput,
	}
	if len(got) != len(want) {
		t.Fatalf("token count: got %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestLexEscapedOperatorsRemainArguments(t *testing.T) {
	tokens := lexTokens(`echo \| \> \{ \$value`)
	if len(tokens) != 6 {
		values := []string{}
		for _, token := range tokens {
			value := ""
			token.Value.IfPresent(func(candidate string) { value = candidate })
			values = append(values, value)
		}
		t.Fatalf("token count: got %d, want 6 (%#v)", len(tokens), values)
	}
	want := []string{"echo", "|", ">", "{", "$value"}
	for i, expected := range want {
		value := ""
		tokens[i].Value.IfPresent(func(candidate string) { value = candidate })
		if value != expected {
			t.Errorf("token %d: got %q, want %q", i, value, expected)
		}
	}
}

func TestLexOperatorsInsideStringRemainStringContent(t *testing.T) {
	tokens := lexTokens(`echo "left|right>file"`)
	if len(tokens) != 3 {
		t.Fatalf("token count: got %d, want 3", len(tokens))
	}
	if tokens[1].Type != String {
		t.Fatalf("quoted operator content token type: got %v, want String", tokens[1].Type)
	}
	value := ""
	hasValue := false
	tokens[1].Value.IfPresent(func(candidate string) {
		value = candidate
		hasValue = true
	})
	if !hasValue || value != "left|right>file" {
		t.Fatalf("quoted operator content: got %q, want %q", value, "left|right>file")
	}
}

func TestParsePipelineKeepsRedirectionsWithCommandStage(t *testing.T) {
	tokens := lexTokens(`echo first > output.txt extra | cat < input.txt`)
	commands, err := parsePipeline(nil, tokens)
	if err != nil {
		t.Fatalf("parsePipeline returned error: %v", err)
	}
	if len(commands) != 2 {
		t.Fatalf("command count: got %d, want 2", len(commands))
	}

	first := commands[0]
	if first.stdoutPath != "output.txt" || first.appendOut {
		t.Fatalf("first redirection: got path %q append %v", first.stdoutPath, first.appendOut)
	}
	if len(first.args) != 3 || first.args[2] != "extra" {
		t.Fatalf("first arguments: got %#v, want echo first extra", first.args)
	}

	second := commands[1]
	if second.stdinPath != "input.txt" {
		t.Fatalf("second input redirection: got %q, want input.txt", second.stdinPath)
	}
	if len(second.args) != 1 || second.args[0] != "cat" {
		t.Fatalf("second arguments: got %#v, want cat", second.args)
	}
}

func TestParsePipelineRejectsMissingRedirectPath(t *testing.T) {
	_, err := parsePipeline(nil, lexTokens(`echo value >`))
	if err == nil {
		t.Fatal("parsePipeline accepted a redirect without a path")
	}
}

func TestParsePipelineRejectsUnmatchedClosingBrace(t *testing.T) {
	_, err := parsePipeline(nil, lexTokens(`echo value }`))
	if err == nil {
		t.Fatal("parsePipeline accepted an unmatched closing brace")
	}
}

func TestParsePipelineRejectsTrailingPipe(t *testing.T) {
	_, err := parsePipeline(nil, lexTokens(`echo value |`))
	if err == nil {
		t.Fatal("parsePipeline accepted a trailing pipe")
	}
}

func TestParsePipelineRejectsRedirectionWithoutStage(t *testing.T) {
	_, err := parsePipeline(nil, lexTokens(`echo value | > output.txt`))
	if err == nil {
		t.Fatal("parsePipeline accepted a redirection without a pipeline stage")
	}
}

func TestParsePipelineExecutesNestedBraceExpression(t *testing.T) {
	commands, err := parsePipeline(nil, lexTokens(`echo { echo nested }`))
	if err != nil {
		t.Fatalf("parsePipeline returned error: %v", err)
	}
	if len(commands) != 1 {
		t.Fatalf("command count: got %d, want 1", len(commands))
	}
	if len(commands[0].args) != 2 || commands[0].args[0] != "echo" || commands[0].args[1] != "nested" {
		t.Fatalf("nested expression arguments: got %#v, want echo nested", commands[0].args)
	}
}

func TestParsePipelineRejectsUnclosedBrace(t *testing.T) {
	_, err := parsePipeline(nil, lexTokens(`echo { echo nested`))
	if err == nil {
		t.Fatal("parsePipeline accepted an unclosed brace")
	}
}
