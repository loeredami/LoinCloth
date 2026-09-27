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

Redirections remain attached to their command stage when they appear between arguments. Blank input is a no-op. A command stage must have a non-empty command name; pipes must have a command on both sides. A stage may specify one input redirection and one output redirection. Repeating either redirection is rejected instead of silently choosing one path. Redirection paths must be a single non-empty word and cannot be shell operators.

## Pipeline execution model

External-only pipelines are connected with operating-system pipes and execute concurrently. A downstream process that closes its input can cause the operating system to deliver a broken-pipe condition to an upstream process; LoinCloth does not currently implement separate upstream cancellation or report a uniform pipeline exit status.

Pipelines containing internal stages, or redirections that require buffering, run sequentially with each stage's output held in memory before the next stage starts. This preserves stage output order but provides no backpressure and can use memory proportional to the largest intermediate output. A failed stage stops the buffered pipeline; in an external-only pipeline, the final stage's failure is reported while earlier stage failures are not exposed as the shell's final status. Mixed-pipeline streaming, bounded buffering, and consistent pipeline exit statuses remain future work.
