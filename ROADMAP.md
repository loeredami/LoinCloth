# LoinCloth work road

## Pre-release 1.4.1

- [x] Accept pasted command batches without dropping lines after the first newline.
- [x] Support fish-style line continuation with a trailing `\\` at the end of a physical line.
- [ ] Add tests for pasted batches, continuation lines, CRLF input, and cancellation while entering a continuation.
- [x] Add initial command piping and redirection support for `|`, `>`, `>>`, and pipeline input via `<`.
- [x] Define initial parsing and execution behavior for chained external pipelines and redirects.
- [x] Preserve multi-line commands in history without offering embedded newlines as ghost completions.
- [x] Add buffered pipeline support for internal commands such as `ls` and `!` commands.
- [ ] Expand pipeline support to handle internal commands and redirections appearing between arbitrary arguments.
  - [x] Add buffered mixed pipelines for internal and external commands.
  - [x] Route Windows built-in output and errors through pipeline writers.
  - [x] Preserve stage-level redirections while parsing arguments.
  - [x] Keep operators inside brace expressions from splitting the outer pipeline.
  - [x] Execute nested brace expressions through the pipeline parser.
  - [ ] Add broader parser coverage for all operator positions and edge cases before marking complete.

## Later

- [ ] Add broader lexer and parser coverage before expanding shell syntax.
  - [x] Add regression coverage for shell operators, quoted operator text, and stage-level redirections.
  - [x] Add regression coverage for nested brace expressions and unclosed braces.
- [x] Manually verify quoted operators, redirections, nested pipelines, and multiline pipeline input in the interactive shell.
- [x] Document interactive input behavior and supported shell operators.
