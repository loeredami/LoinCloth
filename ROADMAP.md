# LoinCloth work road

## v1.4.2 — Experimental 1

This branch experiments with explicit cross-platform administrator execution. The goal is to make elevation visible, opt-in, and handled by the operating system rather than by LoinCloth.

## Phase 0 — Complete deferred v1.4.1 work first

Security changes should not begin until the existing parser and interactive-input behavior is better covered.

- [ ] Complete broader lexer/parser coverage for all operator positions and edge cases.
  - [x] Operators adjacent to quoted and escaped tokens.
  - [x] Nested pipelines and redirections inside brace expressions.
  - [x] Malformed redirections, missing pipeline stages, and unmatched braces.
  - [ ] Empty commands, repeated operators, and source-aware parsing for direct input, `default.cloth`, and nested `!wear` loads.
- [ ] Add automated tests for pasted command batches, continuation lines, CRLF input, and cancellation during continuation input.
  - [x] Add `RunStringTo` integration coverage for pipelines, input redirection, and output redirection.
  - [x] Add non-interactive stdin handling for piped command input.
- [ ] Decide whether internal pipeline stages should remain buffered or gain streaming execution.
  - [ ] Document broken-pipe behavior and upstream cancellation.
  - [ ] Define how mixed internal/external pipelines handle backpressure.
  - [ ] Preserve output ordering and exit status behavior.
- [ ] Establish a baseline regression run before implementing privilege changes.

### Experiment policy

- [ ] Ordinary commands must never be elevated automatically.
- [ ] LoinCloth must not request administrator access during startup, prompt rendering, trust approval, or ordinary command execution.
- [ ] Block switching the shell into superuser/administrator mode through ordinary commands or `sudo` prefixes.
- [ ] Allow persistent administrator mode only through the direct interactive command `!access-administrator`.
- [ ] Leave administrator mode only through the direct interactive command `!exit-administrator` or shell termination.
- [ ] Elevation must be requested explicitly by the user and only through the administrator-mode flow.
- [ ] A process that requests its own privilege mechanism may handle that request itself; LoinCloth must not preemptively request elevation.
- [ ] `!access-administrator` and `!exit-administrator` must be rejected when sourced from any `.cloth` file, including `default.cloth`.
- [ ] These commands must require direct interactive user input and must not be reachable through `!wear`, `!wear-ns`, pipelines, braces, aliases, or configuration loading.
- [ ] LoinCloth must never read, store, echo, or log sudo/UAC credentials.
- [ ] Privilege detection must be informational and must not be treated as authorization.
- [ ] Always use the configured `sudo-prompt` string while administrator mode is active.
- [ ] Do not allow the administrator-mode prompt indicator to be overridden by ordinary prompt changes while the mode is active.
- [ ] Elevated execution must be isolated in a child process.
- [ ] The parent must preserve child stdout, stderr, exit status, cancellation, and failure details.

### Trust-list and source-security experiment

The trust list controls whether an external executable may run without explicit elevation. It is an execution-authorization layer, not an administrator grant. Security decisions must also know where a command came from: direct interactive input, `default.cloth`, or another `.cloth` file.

- [x] Define the initial trust decision engine: trusted and `default.cloth` commands are allowed, interactive unknown commands prompt, and non-interactive unknown commands are denied.
- [x] Wire trust decisions into standalone external command launch and every external pipeline stage.
- [x] Add interactive `Run Once` / `Add command to allow list` / `Do not run` approval handling.
- [x] Deny unknown non-interactive external commands before launch.
- [x] Require approval for commands sourced from non-default `.cloth` files, even when their executable is trusted.
- [x] Add an explicit confirmation step before persisting trust from `!trust`.
- [ ] Define the default policy: deny external executables unless they are trusted or the command is explicitly elevated with `sudo`.
- [ ] Define a native-command policy separate from workspace commands.
- [ ] Allow explicitly approved native operating-system commands to run under the native-command policy.
- [ ] Keep directly typed workspace commands available independently from the external executable trust list.
- [ ] Never treat workspace commands beginning with `!` as native executables.
- [ ] Reject `!` workspace commands from `!trust` entries, native-command entries, and external pipeline trust checks.
- [ ] Allow directly typed workspace commands such as `!wear` under the normal interactive command policy.
- [ ] Define a separate gray-list for commands originating from non-default `.cloth` files.
- [ ] Decide which `!` commands remain available in high-security mode and whether they require a separate explicit policy.
  - [ ] Treat `!wear` as a code-loading operation requiring explicit approval; do not treat it as a trusted native command.
  - [ ] Review `!set`, `!reset`, snapshot, workspace, and configuration-mutating commands separately.
  - [ ] Define whether high-security mode disables workspace command execution by default.
- [ ] Define `sudo command` as a one-invocation elevation request that does not permanently trust the executable.
- [ ] Ensure trust approval and administrator elevation remain separate decisions: `Run Once` must not trigger sudo, and a denied trust check must not silently retry with sudo.
- [ ] If an ordinary child exits with a permission error, report it without automatically retrying or requesting elevation.
- [x] Add direct-interactive trust management commands:
  ```text
  !trust explorer.exe
  !trust explorer
  !trust-list
  !untrust explorer
  ```
- [x] Restrict `!trust` to native/external executable targets; reject targets beginning with `!`.
- [ ] Add confirmation prompts before persisting trust entries.
- [x] Ensure commands loaded from non-default `.cloth` files cannot silently create trusted entries.
- [ ] Define gray-list inspection, approval, revocation, and audit output.
- [ ] Propagate command source through nested `!wear` loads so commands retain their original file context.
- [x] Treat commands loaded from non-default `.cloth` files as gray-listed rather than trusted.
- [x] Prompt when a command is not trusted or is gray-listed:
  1. `Run Once` — execute this invocation without creating a persistent trust entry.
  2. `Add command to allow list` — persist an explicit trust entry.
  3. `Do not run` — deny the invocation.
- [x] Prompt once per external command stage from a gray-listed source.
- [x] Apply the same approval decision independently to each external stage in a pipeline.
- [ ] Never prompt for a password or treat this approval as administrator authorization.
- [x] Default to `Do not run` when no interactive terminal is available.
- [x] Treat commands loaded from `default.cloth` as exempt from the external trusted-list check, while still applying parsing and safety checks.
- [ ] Ensure only `default.cloth` may use `!toggle-security` during configuration loading.
- [ ] Define how security state is restored after a `default.cloth` or gray-listed file finishes loading.
- [ ] Define matching semantics before implementation:
  - [ ] Exact normalized executable path.
  - [ ] Executable basename, such as `explorer.exe` or `explorer`.
  - [ ] Explicit wildcard/glob patterns only when visibly requested.
  - [ ] No implicit substring matching.
- [x] Add an isolated trust-matching prototype with exact-path, basename, and explicit-glob rules.
- [x] Add regression tests for cross-platform basename matching and duplicate trust entries.
- [ ] Require confirmation before adding a trust entry, especially for wildcard or basename entries.
- [ ] Provide commands to inspect and revoke trust entries.
- [ ] Make `Add command to allow list` store the narrowest possible rule, preferring resolved path and executable identity over a broad basename or wildcard.
- [ ] Define behavior when a trusted executable changes, including replacement, symlink, or Windows reparse-point scenarios.
- [ ] Consider storing executable identity with trust entries, such as resolved path plus content hash and optional Windows signature metadata.
- [ ] Invalidate or re-confirm trust when the trusted executable identity changes.
- [ ] Apply trust checks independently to every external stage in a pipeline.
- [ ] Apply native-command policy checks independently to every native stage in a pipeline.
- [ ] Keep workspace `!` commands outside the native executable trust model.
- [ ] Display whether a command was allowed by native policy, user trust, or elevated through `sudo`.
- [ ] Ensure trust does not imply administrator privileges and administrator status does not automatically create trust.

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
- [ ] Display the active configuration path and security source in startup/status output.
- [ ] Reject missing, unreadable, or directory-valued configuration paths before starting the shell.
- [x] Accept `--cloth` only from process arguments, never from a configuration file.
- [ ] Add a development-mode warning when the selected file is outside the protected default configuration location.
- [ ] Add tests for missing paths, unreadable files, directories, source context, and trust inheritance.
- [ ] Add platform-specific trust bootstrap entries to `default.cloth` only for native commands required by the active operating system.
- [ ] Define how platform-specific entries are selected without executing the other platform's commands.
- [ ] Validate that bootstrap entries refer to expected native executables and cannot introduce arbitrary trust entries.
- [ ] Keep platform bootstrap trust separate from user-added executable trust and display its source.
- [ ] Fail closed or warn clearly when required platform bootstrap commands are missing or invalid.
- [ ] Add startup output showing whether trust entries came from platform bootstrap, user trust, or a gray-listed source.

### Security bypass and source controls

- [ ] Add `!toggle-security` as an explicit interactive command that temporarily disables the trusted/native-command checks.
- [ ] Restrict `!toggle-security` inside `.cloth` files; allow it only from the trusted `default.cloth` source.
- [ ] Add `!wear-ns` for explicitly loading/executing a `.cloth` file without the trust-list security checks.
- [ ] Require `!wear-ns` to be typed interactively unless an explicit, separately reviewed policy permits it in `default.cloth`.
- [ ] Display a clear warning whenever security is disabled or `!wear-ns` is used.
- [ ] Do not let security-disabled state silently persist into a new shell session.
- [ ] Record source, security mode, and approval reason in diagnostic output without recording secrets.

#### Trust list versus scopes

- [ ] Keep the security trust list separate from workspace scopes by default.
- [ ] Use a dedicated persistent trust store with explicit ownership and restrictive permissions.
- [x] Add an isolated versioned trust-store persistence prototype with validation, restrictive temporary-file permissions, atomic replacement, and removal support.
- [x] Integrate the persistent store with `!trust`, inspection, and revocation commands.
- [ ] Integrate the persistent store with command authorization.
- [ ] Consider optional scope-local, temporary trust entries only as an isolated future feature.
- [ ] Do not store executable trust entries in `.cloth` files, because those files can be loaded from untrusted projects.
- [ ] Document that scopes manage environment overrides and workspace state, not security authorization.

### Proposed user-facing behavior

- [ ] Add direct-input-only `!access-administrator` to enter administrator mode.
- [ ] Add direct-input-only `!exit-administrator` to leave administrator mode.
- [ ] Make `sudo command arguments` unavailable as a mechanism for switching the persistent shell into administrator mode.
- [ ] Make `sudo command arguments` a LoinCloth-owned built-in on Unix and Windows.
- [ ] Resolve the LoinCloth `sudo` built-in before normal executable lookup, intentionally overriding a native `sudo.exe` with the same command name.
- [ ] Start the elevation request only after parsing, trust checks, and redirection setup are complete and immediately before launching the target command.
- [ ] Never request sudo/UAC merely because LoinCloth started, because a command is untrusted, or because the current shell is non-administrator.
- [ ] Provide an explicit escape hatch for invoking the native executable directly when needed, such as an eventual `command`/absolute-path mechanism.
- [ ] Keep `!elevate -- command arguments` as an optional internal/debug form if it provides useful diagnostics.
- [ ] Reject ambiguous elevation syntax instead of guessing the user's intent.
- [ ] Ensure `sudo` is never inserted automatically when an ordinary command fails.
- [ ] Define clear behavior for `sudo` in pipelines, redirections, and multiline input.
- [ ] Make elevation behavior clear in errors and status output.
- [ ] Define how elevated commands interact with pipelines and redirections.
- [ ] Do not allow an elevated child to silently inherit unsafe shell state or unintended environment overrides.

### Unix implementation — LoinCloth built-in backed by system sudo

- [ ] Resolve the LoinCloth `sudo` built-in before executable lookup.
- [ ] Delegate the built-in to the system `sudo` executable.
- [ ] Do not replace or emulate the system's authentication, policy, timestamp, or logging behavior.

- [ ] Execute explicit elevated commands through the system `sudo` executable only when the user explicitly enters `sudo`.
- [ ] Do not invoke sudo proactively to test whether a command needs privileges.
- [ ] Pass command arguments as an argument vector; do not construct a shell command string.
- [ ] Attach the controlling terminal for interactive sudo authentication and prompts.
- [ ] Preserve stdin, stdout, and stderr without intercepting passwords.
- [ ] Preserve sudo's exit code and signal result.
- [ ] Handle missing sudo, denied authentication, cancellation, and timeout states clearly.
- [ ] Avoid adding `-S`, reading password input, or setting password-related environment variables.
- [ ] Define safe environment behavior for workspace overrides and sensitive variables.

### Windows implementation — LoinCloth built-in backed by UAC

- [ ] Resolve the LoinCloth `sudo` built-in before executable lookup, including when native `sudo.exe` exists.
- [ ] Use native UAC `runas` as the default Windows elevation backend, immediately before an explicitly requested elevated command.
- [ ] Treat native Windows `sudo.exe` as an explicit compatibility option, not the default resolution for `sudo`.
- [ ] Clearly report that the LoinCloth built-in selected the UAC backend.
- [ ] Never request UAC merely because LoinCloth is running unelevated or because an ordinary command failed.
- [ ] Detect the current process token's elevation state through Windows access-token APIs.
- [ ] Distinguish standard user, administrator-not-elevated, elevated administrator, and unknown states.
- [ ] Launch an explicitly elevated child through the native UAC `runas` mechanism.
- [ ] Preserve arguments without unsafe `cmd /c` string construction.
- [ ] Define how the elevated child receives its working directory and environment.
- [ ] Define stdout and stderr forwarding for UAC-launched processes.
- [ ] Return the elevated child exit code to LoinCloth where Windows permits it.
- [ ] Report UAC cancellation separately from command failure.
- [ ] Do not force a global `requireAdministrator` application manifest for ordinary shell use.
- [ ] Document that Windows native sudo availability depends on Windows version and system configuration.

### Privilege display and administrator handling

- [ ] Add a platform-neutral privilege state model for prompt and status display.
- [ ] Track administrator mode separately from one-off elevated child processes.
- [ ] On entering administrator mode, request elevation before changing the shell mode.
- [ ] On failed or cancelled elevation, remain in the normal mode and keep the normal prompt.
- [ ] On exit, restore the normal privilege state and prompt immediately.
- [ ] Prevent configuration files and workspace commands from changing administrator mode.
- [ ] Remove Unix-only privilege calls from shared code paths.
- [ ] Display elevated state without implying that a command will be elevated automatically.
- [ ] Refresh privilege state when relevant process/workspace state changes.
- [ ] Treat unavailable privilege information as unknown, not administrator.

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

- [ ] Define the trust-store location, ownership, permissions, format, locking, atomic updates, backup behavior, and corruption recovery.
- [ ] Require trust-store entries to be validated before use and fail closed if the store is unreadable or modified unexpectedly.
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
- [x] Run the Windows Go test suite under Wine after fixing forward-slash executable path classification; note that Unix-utility-dependent tests and accepted-private-ACL integration skip under Wine.
- [ ] Expand Wine coverage for command parsing, pipelines, redirections, `.cloth` loading, and additional Windows built-ins.
- [ ] Use Wine to verify argument preservation, working-directory handling, stderr behavior, and exit codes.
- [ ] Keep Wine tests separate from native Windows administrator tests.
- [ ] Do not treat Wine as proof of Windows administrator membership, access-token elevation, UAC prompts, or `runas` behavior.
- [ ] Add native Windows CI or manual verification for token classification, UAC approval, UAC cancellation, and elevated child-process behavior.
- [ ] Document required Wine version, prefix setup, and any unsupported security behaviors.

### Verification

- [ ] Add Unix tests using fake sudo executables and controlled PATH values.
- [ ] Add Windows tests for token-state classification.
- [ ] Add Windows manual tests for UAC approval, UAC cancellation, and command failure.
- [ ] Add tests for stdout, stderr, exit codes, signals, and cancellation.
- [ ] Add tests proving ordinary commands are not elevated.
- [ ] Test explicit elevation with pipelines and redirections, or document unsupported combinations.
- [ ] Update README and release notes after the experiment is accepted.

### Experiment exit criteria

- [ ] Unix explicit elevation works without LoinCloth handling passwords.
- [ ] Windows explicit UAC elevation works without globally requiring administrator access.
- [ ] Parent/child I/O and exit behavior is documented and predictable.
- [ ] Ordinary command behavior is unchanged.
- [ ] Security review findings are resolved or explicitly documented.
- [ ] Decide whether the experiment should become the `v1.4.2` release design.


