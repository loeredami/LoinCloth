# Interactive input and shell operators

## Multiline input

A command ending in `\\` continues onto the next physical input line:

```text
echo first \\
echo second
```

The continuation line is displayed with a `> ` prompt. Pasted complete lines are queued and executed one at a time. Multiline history entries are retained, but are not used as multiline ghost completions.

## Operators

LoinCloth supports these operators:

- `command1 | command2` — send the output of one external command to the next command.
- `command > file` — create or truncate a file with command output.
- `command >> file` — append command output to a file.
- `command < file` — use a file as command input.

Operators can be combined:

```text
cat < input.txt | grep error | sort > errors.txt
```

External commands use the workspace environment and can be chained together. Internal commands such as `ls` and `!` commands can participate in buffered pipelines. `cd` cannot be used as a pipeline stage because changing directory only has meaningful state in the shell process itself.

Redirections are being expanded to remain attached to their command stage when they appear between arguments. The parser currently handles common forms such as `echo first > output.txt`, while broader mixed-stage cases remain tracked in the roadmap.
