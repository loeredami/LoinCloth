# DO NOT ADD ADDITIONAL POINTS

# LoinCloth work road

## v1.4.2 — Security work

This roadmap tracks v1.4.2 security work for the release. Trust-list and source-security behavior are product features under active development, not a disposable experiment. Privilege elevation is out of scope for this update.

## Phase 0 — Complete deferred v1.4.1 work first

Security changes should not begin until the existing parser and interactive-input behavior is better covered.

- [x] Complete broader lexer/parser coverage for all operator positions and edge cases.
  - [x] Operators adjacent to quoted and escaped tokens.
  - [x] Nested pipelines and redirections inside brace expressions.
  - [x] Malformed redirections, missing pipeline stages, and unmatched braces.
  - [x] Empty commands, repeated operators, and source-aware parsing for direct input, `default.cloth`, and nested `!wear` loads.
- [x] Add automated tests for pasted command batches, continuation lines, CRLF input, and cancellation during continuation input.
  - [x] Add `RunStringTo` integration coverage for pipelines, input redirection, and output redirection.
  - [x] Add non-interactive stdin handling for piped command input.
- [x] Decide whether internal pipeline stages should remain buffered or gain streaming execution: keep external-only pipelines streamed, and retain sequential buffering for pipelines involving internal stages until a separately tested mixed-stage streaming design is available.
  - [x] Document broken-pipe behavior and upstream cancellation.
  - [x] Define how mixed internal/external pipelines handle backpressure.
  - [x] Document current output ordering and stage-failure reporting behavior.
  - [x] Define and test shell-visible status codes and pipeline exit status behavior: external-only pipelines use the final stage status; buffered pipelines stop and report the first failed stage.
- [x] Establish a baseline regression run before implementing privilege changes (`go test ./...` passed on the feature branch before implementation).

### `ungo` library experiments

- Primary criterion: does an `ungo` API make a real LoinCloth task simpler, clearer, or easier to compose? Prefer useful integrations and ergonomic tests; use benchmarks to resolve a concrete trade-off, not to maximize library usage or chase micro-optimizations.
- [x] Express lexer processing as a named `ungo.PipeSequence`, making the ordered lexing stages visible and individually maintainable instead of rebuilding an inline list of anonymous functions for each input character.
- [x] Runtime-verify the lexer pipeline with a quoted workspace label: `!label` and `!switch "lexer pipeline test"` parsed correctly, switching/closing worked, and `!new w` kept the original workspace active.
- [x] Use `os.LookupEnv` and a single scope traversal for variable lookup, retaining `ungo.SmallMap` scope overrides; measure the change with a 64-scope benchmark.
- [x] Benchmark child-environment assembly with `ungo.SmallMap` against legacy and built-in-map implementations, and verify environment equivalence.
- [x] Benchmark `ungo.SmallMap` command-registry lookup/construction against Go maps; test collision, growth, overwrite, and deletion behavior. Reduce the constructor capacity hint from 256 to 64 based on the measured allocation/build trade-off.
- [x] Measure `ungo.LinkedList` workspace indexing and traversal against slices. At 64 workspaces, last-item lookup measured about 32 ns for the list versus 0.44 ns for a slice; full traversal was about 47 ns versus 26 ns. Keep the benchmark as evidence for a future storage decision rather than making an unrelated workspace rewrite now.
- [x] Review `ungo.Optional` behavior: `Value()` returns the zero value when absent rather than reporting misuse. Guard workspace/environment access before extracting values, and test empty lists, invalid indexes, nil workspaces, and missing scope lists so malformed state returns errors/empty optionals rather than panicking or succeeding silently.
- [x] Benchmark `ungo.Queue` for pasted command batches against slice-cursor and string-buffer representations; verify FIFO, empty values, and empty-queue behavior. For 700 commands, the linked queue used 700 allocations/16.8 KB and took about 11.3 us; at 5,000 commands it used 5,000 allocations/120 KB and took about 83 us. The alternatives were materially cheaper in allocations, so keep the current input buffer and do not migrate to one queue node per command. This microbenchmark does not settle chunk-queue memory retention or CRLF/continuation integration.
- [x] Runtime-verify scope creation, mutation, and `!snapshot` serialization; the saved `.cloth` reproduced the expected scope and override commands.
- [x] Review `ungo` concurrency candidates before integration: `Worker.Cancel` only sets a flag the worker never checks; worker result/running state and the global registry are unsynchronized; `Promise` uses a consuming channel and `Reject` type-asserts `error` to `T`; `EventLoop` has close/restart and blocking-post lifecycle hazards. Do not adopt these primitives in shell execution without fixing and testing their contracts in `ungo`.
- [x] Review `ungo.PipeSequence` as an ergonomic fit for lexing; do not use `ungo.Pipeline` for command execution because its `T -> T` stages cannot express process errors, exit status, cancellation, or I/O ownership.
- [x] Review `ungo.Specification`, `Exception`, and `ServiceRegistry` as potential conveniences. `Specification` predicate fields lack exported constructors, `Exception` largely wraps Go's existing `(value, error)` flow, and `ServiceRegistry.Add` exits the process on initialization errors while shutdown errors are discarded; none currently improve LoinCloth code safely.
### Trust-list and source-security 

The trust list controls whether an external executable may run without explicit elevation. It is an execution-authorization layer, not an administrator grant. Security decisions must also know where a command came from: direct interactive input, `default.cloth`, or another `.cloth` file.

- [x] Define the initial trust decision engine: trusted and `default.cloth` commands are allowed, interactive unknown commands prompt, and non-interactive unknown commands are denied.
- [x] Wire trust decisions into standalone external command launch and every external pipeline stage.
- [x] Add interactive `Run Once` / `Trust for this session` / `Do not run` approval handling.
- [x] Deny unknown non-interactive external commands before launch.
- [x] Require approval for commands sourced from non-default `.cloth` files, even when their executable is trusted.
- [x] Add an explicit confirmation step before adding session trust from `!trust`.
- [x] Define the default policy: deny external executables unless they are trusted.
- [x] Define a native-command policy separate from workspace commands.
- [x] Allow explicitly approved native operating-system commands to run under the native-command policy.
- [x] Keep directly typed workspace commands available independently from the external executable trust list.
- [x] Never treat workspace commands beginning with `!` as native executables.
- [x] Reject `!` workspace commands from `!trust` entries, native-command entries, and external pipeline trust checks.
- [x] Allow directly typed workspace commands such as `!wear` under the normal interactive command policy.
- [x] Define a separate gray-list for commands originating from non-default `.cloth` files.
- [ ] Decide which `!` commands remain available in high-security mode and whether they require a separate explicit policy.
  - [ ] Treat `!wear` as a code-loading operation requiring explicit approval; do not treat it as a trusted native command.
  - [ ] Review `!set`, `!reset`, snapshot, workspace, and configuration-mutating commands separately.
  - [ ] Define whether high-security mode disables workspace command execution by default.
- [x] Add direct-interactive trust management commands:
  ```text
  !trust explorer.exe
  !trust explorer
  !trust-list
  !untrust explorer
  ```
- [x] Restrict `!trust` to native/external executable targets; reject targets beginning with `!`.
- [x] Reject `!` workspace command rules in session trust insertion and matching.
- [x] Require confirmation before adding a trust rule to the current session.
- [x] Ensure commands loaded from non-default `.cloth` files cannot silently create trusted entries.
- [ ] Define gray-list inspection, approval, revocation, and audit output.
- [x] Propagate command source through nested `!wear` loads so commands retain their original file context.
- [x] Treat commands loaded from non-default `.cloth` files as gray-listed rather than trusted.
- [x] Prompt when a command is not trusted or is gray-listed:
  1. `Run Once` — execute this invocation without changing session trust.
  2. `Trust for this session` — add an explicit rule to this Loin process's in-memory trust list.
  3. `Do not run` — deny the invocation.
- [x] Prompt once per external command stage from a gray-listed source.
- [x] Apply the same approval decision independently to each external stage in a pipeline.
- [x] Never prompt for a password or treat this approval as administrator authorization.
- [x] Default to `Do not run` when no interactive terminal is available.
- [x] Treat commands loaded from `default.cloth` as exempt from the external trusted-list check, while still applying parsing and safety checks.
- [x] Allow validated, protected `default.cloth` `!trust` directives to seed the current session's in-memory trust list; do not persist them outside the session.
- [ ] Define how security state is restored after a `default.cloth` or gray-listed file finishes loading.
- [x] Define matching semantics before implementation:
  - [x] Exact normalized executable path.
  - [x] Executable basename, such as `explorer.exe` or `explorer`.
  - [x] Explicit wildcard/glob patterns only when visibly requested.
  - [x] No implicit substring matching.
- [x] Add an isolated trust-matching prototype with exact-path, basename, and explicit-glob rules.
- [x] Add regression tests for cross-platform basename matching and duplicate trust entries.
- [x] Require explicit confirmation before adding an executable rule to the current session's trust list; declining denies the command.
- [x] Provide direct-interactive commands to inspect and revoke current-session trust entries.
- [x] Make `Trust for this session` use the narrowest possible rule, preferring resolved path and executable identity over a broad basename or wildcard.
- [ ] Define behavior when a trusted executable changes, including replacement, symlink, or Windows reparse-point scenarios.
- [ ] Consider storing executable identity with trust entries, such as resolved path plus content hash and optional Windows signature metadata.
- [ ] Invalidate or re-confirm trust when the trusted executable identity changes.
- [x] Apply trust checks independently to every external stage in a pipeline.
- [x] Apply native-command policy checks independently to every native stage in a pipeline.
- [x] Keep workspace `!` commands outside the native executable trust model.
- [ ] Display whether a command was allowed by native policy, user trust, or elevated through `sudo`.
- [x] Ensure trust does not imply administrator privileges and administrator status does not automatically create trust.

### Configuration selection and development mode

- [x] Add a startup flag for selecting an alternate `.cloth` configuration:
  ```text
  loin --cloth path/to/development.cloth
  ```
- [x] Define the initial behavior: `--cloth` replaces `default.cloth` for that process.
- [x] Add a repository-local `default.cloth` for deterministic development testing.
- [x] Make the selected file's source context explicit: a development file must not inherit `default.cloth` trust automatically.
- [x] Treat an explicitly selected development `.cloth` as gray-listed by default.
- [x] On Unix, restrict the normal `.loin` config directory to `0700` and default cloth to `0600` before granting its source exemption; reject symlinks, non-regular files, and files/directories not owned by the current user.
- [x] If default-cloth protection validation fails, load it as gray-listed; fail closed on Windows when ACL validation is unavailable or rejects the ACL.
- [x] Validate Windows default-cloth directory/file handles: require current-user ownership, an explicit DACL granting access only to the current user, SYSTEM, or local Administrators, and reject reparse points or unsupported ACE types.
  - [x] Fail closed to gray-listing when ACL inspection fails or an unapproved SID (including Everyone) is granted access.
  - [x] Add Windows ACE-policy and SID-matching tests and run them under Wine.
  - [ ] Verify the accepted/rejected ACL cases on native Windows; Wine ACL behavior is not authoritative.
- [x] Display the active configuration path and security source in startup/status output.
- [x] Reject missing, unreadable, or directory-valued configuration paths before starting the shell.
- [x] Accept `--cloth` only from process arguments, never from a configuration file.
- [x] Add a development-mode warning when the selected file is outside the protected default configuration location.
- [x] Add tests for missing paths, unreadable files, directories, source context, and trust inheritance.
- [x] Add platform-specific trust bootstrap entries to `default.cloth` only for native commands required by the active operating system.
- [x] Define how platform-specific entries are selected without executing the other platform's commands.
- [x] Validate that bootstrap entries refer to expected native executables and cannot introduce arbitrary trust entries.
- [x] Keep platform bootstrap trust separate from user-added executable trust and display its source.
- [x] Fail closed or warn clearly when required platform bootstrap commands are missing or invalid.
- [x] Add startup output showing whether trust entries came from platform bootstrap, user trust, or a gray-listed source.

### Security bypass and source controls

- [ ] Add `!toggle-security` as an explicit interactive command that temporarily disables the trusted/native-command checks.
- [ ] Restrict `!toggle-security` inside `.cloth` files; allow it only from the trusted `default.cloth` source.
- [ ] Add `!wear-ns` for explicitly loading/executing a `.cloth` file without the trust-list security checks.
- [ ] Require `!wear-ns` to be typed interactively unless an explicit, separately reviewed policy permits it in `default.cloth`.
- [ ] Display a clear warning whenever security is disabled or `!wear-ns` is used.
- [ ] Do not let security-disabled state silently persist into a new shell session.
- [ ] Record source, security mode, and approval reason in diagnostic output without recording secrets.

#### Trust list versus scopes

- [x] Keep executable trust separate from workspace scopes and session-local by default.
- [x] Remove the unused persistent trust-store prototype so approvals cannot accidentally be shared across Loin launches.
- [x] Keep `!trust`, trust inspection, and revocation session-local.
- [x] Never persist executable trust across Loin launches; session approvals and `default.cloth` seeds exist only in the current process memory and are discarded on exit.
- [ ] Consider optional scope-local, temporary trust entries only as an isolated future feature (still must not outlive the Loin process).
- [x] Do not allow arbitrary `.cloth` files to add trust; the validated, protected `default.cloth` is the only file allowed to seed in-memory session trust.
- [x] Document that scopes manage environment overrides and workspace state, not security authorization.

### Recommended security additions for review

- [ ] Write a threat model covering malicious `.cloth` files, untrusted workspaces, PATH hijacking, executable replacement, symlink/reparse-point attacks, and compromised configuration files.

### Protected configuration and process file-access monitoring

`default.cloth` and files it explicitly loads should be treated as protected configuration. A child process must not be assumed safe merely because it was launched by LoinCloth.

- [ ] Define the protected file set beginning with `default.cloth` and every file it loads directly or indirectly.
- [ ] Track the process tree and retain the source context for every child process.
- [ ] Detect attempts by launched processes to read, write, delete, rename, or execute protected files.
- [ ] Where the operating system permits pre-access enforcement, pause the access and prompt before allowing it.
- [ ] Prompt with:
  1. The process identity and executable path.
  2. The protected file being accessed.
  3. The requested operation.
  4. The source command or configuration context.
  5. The security threat the access could impose.
  6. `Allow Once`, `Allow for This Process`, `Allow Persistently`, or `Deny`.
- [ ] Default to `Deny` when access cannot be evaluated safely or no interactive terminal is available.
- [ ] Never expose protected-file contents in prompts, logs, or diagnostics.
- [ ] Keep process file-access permissions separate from executable trust permissions.
- [ ] Ensure `sudo` or administrator mode does not silently bypass protected-file prompts.
- [ ] Define whether explicitly trusted processes may still require protected-file approval.
- [ ] Record protected-file decisions without recording secrets.
- [ ] Provide a status/revocation mechanism for process-specific and persistent file-access approvals.
- [ ] Unix feasibility review: evaluate audit/fanotify or another OS-supported pre-access mechanism; ordinary polling is insufficient for enforcement.
- [ ] Windows feasibility review: evaluate supported file-system/process monitoring APIs or a service/minifilter architecture; ordinary directory watchers are insufficient for enforcement.
- [ ] If pre-access enforcement is unavailable, fail closed or clearly label the feature as audit-only rather than claiming it provides protection.

- [x] Keep executable approval trust local to each Loin process; do not load shared or persistent entries into a running session.
- [ ] Resolve executable identity immediately before launch and define protections against time-of-check/time-of-use replacement.
- [ ] Define whether trust is based on path, file identity, content hash, publisher/signature, or a combination of these.
- [ ] Sanitize privileged child environments by default; explicitly define which variables and workspace overrides may cross the elevation boundary.
- [ ] Prevent untrusted working directories and PATH entries from influencing privileged command resolution.
- [ ] Define whether an elevated command may write to a path selected by an untrusted source file.
- [ ] Require explicit per-stage decisions for elevated commands inside pipelines, or forbid mixed-privilege pipelines initially.
- [ ] Add a safe recovery path if administrator-mode startup, handoff, or shutdown fails.
- [ ] Define cancellation and emergency-exit behavior while an elevated child or trust prompt is active.
- [ ] Avoid making the configurable `sudo-prompt` string the only administrator indicator; add a fixed, non-configurable elevated-state marker.
- [ ] Define privacy-preserving audit events for trust decisions without storing command secrets or credentials.
- [ ] Add a security mode/status command that reports source context, trust state, and privilege state without exposing sensitive environment data.

### Security review

- [ ] Review command parsing, quoting, braces, pipelines, and redirections for injection paths.
- [ ] Review environment inheritance and workspace variable overrides for privilege escalation risks.
- [ ] Review current Windows built-ins for path traversal, overwrite, and permission behavior.
- [ ] Review symlink and Windows reparse-point behavior in file operations.
- [ ] Ensure errors never include passwords, tokens, or sensitive environment values.
- [ ] Document the trust boundary between the parent shell and elevated child.

### Cross-platform test environments

- [ ] Add a Wine-based Windows smoke-test environment for Linux development.
  - [x] Install Wine 11.18 and initialize a user-owned prefix for local smoke testing.
  - [x] Run the Windows amd64 build under Wine with piped `echo`, `mkdir`, and `rm` commands; verify the temporary directory is removed.
- [x] Exercise default-config startup under Wine and verify an Everyone-accessible config directory falls back to gray-listing.
- [x] Run Windows ACE-policy and SID-matching tests under Wine.
- [x] Run the Windows Go test suite under Wine after fixing forward-slash executable path classification; note that Unix-utility-dependent tests and the private-current-ACL integration skip under Wine when the temp directory grants access to Everyone.
- [ ] Expand Wine coverage for command parsing, pipelines, redirections, `.cloth` loading, and additional Windows built-ins.
- [ ] Use Wine to verify argument preservation, working-directory handling, stderr behavior, and exit codes.
- [ ] Keep Wine tests separate from native Windows administrator tests.
- [ ] Do not treat Wine as proof of Windows administrator membership, access-token elevation, UAC prompts, or `runas` behavior.
- [ ] Add native Windows CI or manual verification for token classification, UAC approval, UAC cancellation, and elevated child-process behavior.
- [ ] Document required Wine version, prefix setup, and any unsupported security behaviors.
