# LoinCloth

LoinCloth is a shell program, built for project management and multi tasking.

You can create multiple workspaces, and scopes which each hold their own aliases.

You can create and load in .cloth scripts, to load in scope and aliases.

# Workspaces

```sh
~/Projects/2026/March/LoinCloth (main) ~811.145µs
» !new w
[0] ~/Projects/2026/March/LoinCloth [*] (main) ~4.73µs
[1] ~/Projects/2026/March/LoinCloth [H]
»
```

`!new w` creates a new space.

The `[*]` indicates to us the current workspace in which we are in, in our case we are at workspace with index 0.
The `[H]` indicates a workspace that is open in the same folder as our current workspace.

The `~4.73µs` is the rough execution time the command took, including command parsing time.

The `(main)` is our current git branch, this will not display if there is no branch detected **in the active workspace folder**. Also, yes I am commiting to main, fight me.

Use `!switch <index>` to switch to our new workspace.

```sh
~/Projects/2026/March/LoinCloth (main) ~811.145µs
» !new w
[0] ~/Projects/2026/March/LoinCloth [*] (main) ~4.73µs
[1] ~/Projects/2026/March/LoinCloth [H]
» !switch 1
[0] ~/Projects/2026/March/LoinCloth [H]
[1] ~/Projects/2026/March/LoinCloth [*] (main) ~10.71µs
»
```

You can see, we are now in workspace `[1]`. 
We will now see the `[H]` disappear in index `[0]` once we change directories.

```sh
[0] ~/Projects/2026/March/LoinCloth [H]
[1] ~/Projects/2026/March/LoinCloth [*] (main) ~10.71µs
» cd ..
[0] ~/Projects/2026/March/LoinCloth
[1] ~/Projects/2026/March [*] ~27.02µs
»
```

We also have a typical ls command, which overrides your os ls, this was added because windows doesn't have one, and I had to make sure it was working for cross compatibility.

```sh
[0] ~/Projects/2026/March/LoinCloth [H]
[1] ~/Projects/2026/March/LoinCloth [*] (main) ~17.01µs
» ls

.git/               .gitignore          LICENSE             README.md
build_all.sh*       builds/             command_lexer.go    go.mod
go.sum              main.go             terminal_unix.go    terminal_windows.go
test.cloth          workspace_commands.go

[0] ~/Projects/2026/March/LoinCloth [H]
[1] ~/Projects/2026/March/LoinCloth [*] (main) ~105.981µs
»
```

Anyways, we no longer need workspace `[0]`, so let's use the `!close` command to close it.

```sh
[0] ~/Projects/2026/March/LoinCloth [H]
[1] ~/Projects/2026/March/LoinCloth [*] (main) ~805.365µs
» !close 0
~/Projects/2026/March/LoinCloth (main) ~6.32µs
»
```

# Scopes
Scopes are used for saving temporary environment variables.

To start let's create a scope called "builder" like so:

```sh
~/Projects/2026/March/LoinCloth (main) ~844.463µs
» !new s builder
~/Projects/2026/March/LoinCloth (main) ~6.25µs
:builder»
```

The prompt will now prefix with the series of scopes in order of which it overrides your env variables.

**Note that these temp variables will also auto replace as arguments.**

You can also stack scopes, do not worry about your scope names disappearing once you start typing, this is intentional design to prevent line noise.

```sh
~/Projects/2026/March/LoinCloth (main)
» !new s builder
~/Projects/2026/March/LoinCloth (main) ~8.84µs
» !new s r
~/Projects/2026/March/LoinCloth (main) ~10.22µs
:builder:r»
```

Now, if you have any variables declared in r, which also exists in builder, the varaibles in r will ahve priority over builder.

You can now use the `!set <key> <value>` command like so:

```sh
~/Projects/2026/March/LoinCloth (main) ~10.22µs
» !set GOPROXY direct
~/Projects/2026/March/LoinCloth (main) ~5.95µs
» !set build "go build ."
~/Projects/2026/March/LoinCloth (main) ~8.51µs
» build
~/Projects/2026/March/LoinCloth (main) ~33.449099ms
:builder:r»
```

Note that the variables you declare only exist in each scope, once you drop them like in the example below, they will no longer exist.

```sh
~/Projects/2026/March/LoinCloth (main) ~8.51µs
» build
~/Projects/2026/March/LoinCloth (main) ~33.449099ms
» !drop r
~/Projects/2026/March/LoinCloth (main) ~6.83µs
» build
exec: "build": executable file not found in $PATH
~/Projects/2026/March/LoinCloth (main) ~140.141µs
:builder»
```

If you want each scope to always have a specific set of variables, consider writing a cloth file.

test.cloth:
```sh
!new s go-builder
!set run "go run ."
!set buid "go build ."
!set GOPROXY direct
!set update "go get -u"
```

Now you can wear the cloth:
```sh
~/Projects/2026/March/LoinCloth (main) ~103.57µs
» !wear test.cloth
~/Projects/2026/March/LoinCloth (main) ~57.05µs
:go-builder»
```

and run it's commands:

```sh
~/Projects/2026/March/LoinCloth (main) ~57.05µs
» update github.com/loeredami/ungo@latest
~/Projects/2026/March/LoinCloth (main) ~314.921787ms
:go-builder»
```

This ran `go get -u github.com/loeredami/ungo@latest`, with the `GOPROXY` set to `direct`.

`!drop go-builder`, to drop the builder commands and variables


# Other
## Comments

Comments start and end with `##`

example:
```sh
~/Projects/2026/March/LoinCloth (main) ~877.004µs
» echo Hello ## I am a comment, I will not be printed out. ## World
Hello World
~/Projects/2026/March/LoinCloth (main) ~557.296µs
»
```
I really need custom syntax highlighting for this.
