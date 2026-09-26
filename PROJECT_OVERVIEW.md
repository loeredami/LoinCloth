# LoinCloth project overview

This file is a compact orientation guide for contributors and coding agents.

## Project purpose

LoinCloth is a Go-based interactive shell focused on project workspaces, scopes, `.cloth` configuration files, command pipelines, and cross-platform terminal behavior.

The project currently has two related goals:

1. Maintain the v1.4.1 shell functionality.
2. Experiment with high-security command authorization and cross-platform administrator handling for v1.4.2.

## Current Git state

The active development line is:

```text
v1.4.2-trust-enforcement
```

The current security branch is based on the following progression:

```text
main
└── v1.4.2
    └── v1.4.2-trust-source-foundation
        └── v1.4.2-trust-list-foundation
            └── v1.4.2-trust-store-persistence
                └── v1.4.2-trust-enforcement
```

Branches are intentionally used for potentially breaking security changes. After validation, feature branches are merged into `v1.4.2`, pushed, and then used as the base for the next isolated experiment.

## Important files

| File | Purpose |
| --- | --- |
| `main.go` | Application startup, command execution, pipelines, redirections, configuration loading, and non-interactive input. |
| `terminal_helpers.go` | Raw input editing, history, completion, multiline continuation, and prompt rendering. |
| `terminal_unix.go` | Unix raw-terminal behavior and platform stubs. |
| `terminal_windows.go` | Windows terminal setup and Windows built-ins. |
| `command_lexer.go` | Lexer for identifiers, paths, strings, variables, braces, pipes, and redirection operators. |
| `command_lexer_test.go` | Lexer and parser regression tests. |
| `run_string_test.go` | Integration tests for command strings, pipelines, and redirections. |
| `workspace.go` | `State`, workspaces, scopes, configuration path, and command-source state. |
| `workspace_commands.go` | Workspace and `!` command implementations. |
| `command_source.go` | Command-origin classification: interactive input, `default.cloth`, `.cloth`, and development cloth. |
| `trust_store.go` | Trust-rule matching and persistent trust-store prototype. |
| `trust_policy.go` | Pure trust decision policy: allow, prompt, or deny. |
| `trust_*_test.go` | Trust matching, persistence, and policy tests. |
| `default.cloth` | Repository-local development configuration. |
| `run_dev.sh` | Builds `loin-dev` and runs it with the repository `default.cloth`. |
| `ROADMAP.md` | Active v1.4.2 security, privilege, trust, and testing plan. |
| `SHELL_OPERATORS.md` | Pipeline, redirection, and multiline input documentation. |

## Command execution flow

1. Input is received interactively, from a pasted batch, from a `.cloth` file, or from non-interactive stdin.
2. The command is lexed by `Lex`.
3. `parsePipeline` groups command stages and attaches redirection metadata.
4. `processTokens` expands variables, paths, braces, and nested expressions.
5. `runPipeline` executes external and supported internal stages.
6. `RunStringFromSource` preserves command-origin context for future trust and gray-list decisions.

## Supported shell behavior

Current shell syntax includes:

```text
command1 | command2
command > file
command >> file
command < file
```

Multiline continuation is supported with a trailing backslash:

```text
printf "zulu\nalpha\n" | \\
sort
```

Pasted complete lines are queued rather than discarded. Non-interactive stdin is processed line by line and supports continuation lines.

Brace expressions are parsed through the pipeline parser, allowing nested forms such as:

```text
echo { ls | grep .go }
```

## Development workflow

Run the repository development shell:

```sh
./run_dev.sh
```

Select another development configuration:

```sh
go run . --cloth path/to/development.cloth
```

Run standard validation:

```sh
go test ./...
go build .
./build_all.sh
```

The normal smoke test should also exercise the development launcher:

```sh
printf '%s\n' 'echo dev' 'printf "zulu\nalpha\n" | sort' | ./run_dev.sh
```

`run_dev.sh` creates `loin-dev`. Remove that generated artifact before committing if it is not ignored.

## Security work status

The following foundations exist:

- Command-source tracking.
- Repository-local development configuration selection with `--cloth`.
- Trust-rule matching for exact paths, basenames, and explicit globs.
- Versioned persistent trust-store prototype with validation and atomic writes.
- Direct-interactive trust management commands: `!trust`, `!trust-list`, and `!untrust`.
- Pure trust decision policy:
  - Trusted executable: allow.
  - `default.cloth`: allow under the current prototype policy.
  - Unknown interactive command: prompt.
  - Unknown non-interactive command: deny.

The following are not complete:

- Trust prompt integration with actual command launching.
- Gray-list approval for non-default `.cloth` files.
- `!toggle-security` and `!wear-ns`.
- `!access-administrator` and `!exit-administrator`.
- Unix `sudo` delegation.
- Windows UAC `runas` handling.
- Protected `default.cloth` process/file-access monitoring.

Do not describe the current prototype as a complete security boundary. The trust policy is not yet wired into all process launches.

## Security design principles

- Never capture, store, echo, or log sudo/UAC passwords.
- Never elevate ordinary commands automatically.
- Keep trust authorization separate from administrator authorization.
- Treat commands from non-default `.cloth` files as gray-listed.
- Do not treat `!` workspace commands as native executables.
- Default to deny when an authorization decision cannot be made safely.
- Keep trust storage separate from workspace scopes.
- Prefer narrow executable identity rules over broad basename or wildcard rules.
- Use OS-native privilege mechanisms rather than emulating authentication.

## Branch and commit policy

- Create a dedicated branch for potentially breaking changes.
- Run Go tests, native build, build matrix, and `run_dev.sh` smoke tests.
- Commit only after the branch is validated.
- Push every commit after creating it.
- Merge validated feature branches into `v1.4.2` before starting the next major experiment.
- Keep generated binaries and temporary test files out of commits.
