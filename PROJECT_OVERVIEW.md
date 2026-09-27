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
v1.4.2-session-scoped-trust
```

The current security branch is based on the following progression:

```text
main
└── v1.4.2
    └── v1.4.2-trust-source-foundation
        └── v1.4.2-trust-list-foundation
            └── v1.4.2-trust-store-persistence
                └── v1.4.2-trust-enforcement
                    └── v1.4.2-trust-launch-enforcement
                        └── v1.4.2-trust-confirmation
                            └── v1.4.2-default-cloth-hardening
                                └── v1.4.2-windows-default-cloth-acl
                                    └── v1.4.2-security-status
                                        └── v1.4.2-parser-input-coverage
                                            └── v1.4.2-readme-current-behavior
                                                └── v1.4.2-pipeline-status
                                                    └── v1.4.2-confirm-prompt-persistence
                                                        └── v1.4.2-env-scope-lookup
                                                            └── v1.4.2-child-environment-map
                                                                └── v1.4.2-smallmap-child-environment
                                                                    └── v1.4.2-reject-workspace-trust-rules
                                                                        └── v1.4.2-trust-store-fail-closed
                                                                            └── v1.4.2-trust-store-change-detection
                                                                                └── v1.4.2-trust-store-recovery
                                                                                    └── v1.4.2-trust-store-permissions
                                                                                        └── v1.4.2-trust-store-locking
                                                                                            └── v1.4.2-session-scoped-trust
                                                                                                └── v1.4.2-remove-persistent-trust-prototype (active)
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
| `terminal_input_test.go` | Pasted batches, CRLF, continuation, cancellation, and command-source regression tests. |
| `run_string_test.go` | Integration tests for command strings, pipelines, and redirections. |
| `workspace.go` | `State`, workspaces, scopes, configuration path, and command-source state. |
| `workspace_commands.go` | Workspace and `!` command implementations, including environment and scope variable lookup. |
| `workspace_commands_test.go` | Scope/environment lookup regression tests and a many-scope benchmark. |
| `command_source.go` | Command-origin classification: interactive input, `default.cloth`, `.cloth`, and development cloth. |
| `trust_store.go` | In-memory trust-rule matching for the current session. |
| `trust_policy.go` | Pure trust decision policy: allow, prompt, or deny. |
| `trust_*_test.go` | Session trust matching, prompt, and policy tests. |
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
go test -run '^$' -bench '^BenchmarkGetEnvValueManyScopes$' -benchmem
```

The normal smoke test should also exercise the development launcher, including allowed internal commands and expected non-interactive external-command denial:

```sh
printf '%s\n' 'ls' 'echo external-command-is-denied-unless-trusted' | ./run_dev.sh
```

`run_dev.sh` creates `loin-dev`. Remove that generated artifact before committing if it is not ignored.

A basic Windows runtime smoke test is available on Linux after installing Wine. It exercises the Windows amd64 binary with piped input and Windows built-ins:

```sh
printf '%s\n' 'mkdir wine-smoke-temp' 'echo wine-builtins-ok' 'rm wine-smoke-temp' 'exit' | \
  WINEDEBUG=-all WINEPREFIX="$HOME/.local/share/loin-wine-prefix" \
  wine builds/loin_windows_amd64.exe --cloth default.cloth
```

Run the Windows-targeted Go tests under Wine with:

```sh
GOOS=windows GOARCH=amd64 \
  WINEPREFIX="$HOME/.local/share/loin-wine-prefix" WINEDEBUG=-all \
  go test -exec wine ./...
```

Wine can catch basic startup and command-processing regressions; it does not validate native Windows ACL semantics, access tokens, UAC, or `runas` behavior. In the current Wine prefix, the default config directory grants access to `Everyone`, so LoinCloth correctly treats it as gray-listed. The test that reads a file under a private current-user ACL skips if the runtime temp directory grants access to an untrusted SID; tests requiring Unix utilities also skip under Wine. The synthetic DACL policy tests pass, but native Windows verification of actual ACL handling is still needed.

## Security work status

The following foundations exist:

- Parser rejects empty command names, malformed/repeated redirections, and pipe stages without commands; blank input remains a no-op.
- `!last-status` reports the previous non-empty command's status. External-only pipelines use the final stage's status; buffered pipelines stop at the first failed stage.
- Scope overrides use `ungo.SmallMap`; environment lookup uses `os.LookupEnv` and visits scope overrides in one forward pass, preserving newest-scope precedence. The 64-scope benchmark dropped from about 4.25 us/92 allocations to 0.36 us/6 allocations per lookup on the development machine; rerun the benchmark for local results.
- Child process environments are assembled with `ungo.SmallMap` while preserving host environment values and applying scope overrides in order. In fair local benchmarks (same parsing and output construction), this took about 6.5 us/88 allocations versus 7.5 us/90 allocations for a built-in map, with about 6 KB more temporary allocation. The legacy path measured about 18 us/345 allocations. Rerun `go test -run '^$' -bench '^BenchmarkCommandEnvironment' -benchmem` to compare locally.
- Pasted interactive batches preserve commands across the input buffer boundary; continuation, CRLF, and Ctrl+C cancellation are covered by tests.
- Nested `!wear` command source is tracked and restored, including loads initiated by the default-configuration source.
- Command-source tracking.
- Repository-local development configuration selection with `--cloth`.
- Trust-rule matching for exact paths, basenames, and explicit globs.
- Runtime trust rules exist only in memory for the current Loin process and are discarded on exit; prior persistent-store prototype code has been removed to avoid implying runtime approvals are shared.
- Workspace commands beginning with `!` are rejected as executable trust rules.
- Direct-interactive trust management commands: `!trust`, `!trust-list`, and `!untrust`.
- `!trust` requires a separate interactive confirmation before adding a rule for the current session; `!trust-list` and `!untrust` inspect and revoke only current-session rules.
- Choosing “Trust for this session” at an execution prompt requires a second explicit confirmation; declining prevents that command's launch.
- `!security-status` reports the active configuration path and source, configuration trust, current-session trust-rule count, and privilege state.
- Explicitly selected `.cloth` files are validated as readable regular files and produce a development-mode warning when outside the protected default location.
- Launch-time checks for standalone external commands and external stages in pipelines.
- Interactive approval choices: Run Once, Trust for this session, or Do not run. Prompts are written to stderr so they do not become redirected command output.
- Unknown non-interactive external commands are denied before launch.
- Non-default `.cloth` commands are gray-listed and require interactive approval, even if the executable itself is trusted.
- Piped stdin is marked non-interactive; it cannot run trust-management commands or approve executables.
- Trust checks happen before single-command redirection files are created or truncated.
- `default.cloth` commands receive the executable-trust exemption only after platform-specific validation: Unix ownership, regular-file, no-symlink, and private-permission checks (tightened to `0700`/`0600` where possible); Windows current-user ownership, explicit DACL validation, and reparse-point rejection.
- Windows permits DACL access only for the current user, SYSTEM, and local Administrators. Untrusted SIDs, unsupported ACEs, or ACL inspection failures fall back to gray-listing.

The following are not complete:

- Native-command policy and comprehensive review/testing for every mixed/internal pipeline path.
- Native Windows verification of the default-cloth ACL acceptance/rejection cases; Wine’s ACL behavior is not authoritative.
- `!toggle-security` and `!wear-ns`.
- `!access-administrator` and `!exit-administrator`.
- Unix `sudo` delegation.
- Windows UAC `runas` handling.
- Protected `default.cloth` process/file-access monitoring.

Do not describe the current prototype as a complete security boundary. Native-command policy, source-context edge cases, native Windows ACL validation, and privilege elevation are still incomplete.

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
- Commit and push feature branches after automated validation; do not merge into `v1.4.2` until the user confirms the requested runtime checks passed.
- Push every commit after creating it.
- Merge validated feature branches into `v1.4.2` before starting the next major experiment.
- Keep generated binaries and temporary test files out of commits.
