package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

func authorizeExecutable(state *State, executablePath string, _ io.Writer) bool {
	writer := bufio.NewWriter(os.Stderr)
	if state == nil {
		fmt.Fprintln(writer, "blocked untrusted executable: no security state available")
		writer.Flush()
		return false
	}

	decision := EvaluateTrust(state.trustStore, executablePath, state.commandSource, state.interactiveInput)
	switch decision {
	case TrustAllow:
		return true
	case TrustDeny:
		fmt.Fprintf(writer, "blocked untrusted executable in non-interactive input: %s\n", executablePath)
		writer.Flush()
		return false
	case TrustPrompt:
		return promptExecutableTrust(state, executablePath, writer)
	default:
		return false
	}
}

func promptExecutableTrust(state *State, executablePath string, output *bufio.Writer) bool {
	fmt.Fprintf(output, "Untrusted executable: %s\n", executablePath)
	fmt.Fprintln(output, "1. Run Once")
	fmt.Fprintln(output, "2. Trust for this session")
	fmt.Fprintln(output, "3. Do not run")
	fmt.Fprint(output, "Choose [1-3]: ")
	output.Flush()

	var choice string
	if _, err := fmt.Fscanln(os.Stdin, &choice); err != nil {
		fmt.Fprintln(output, "no interactive approval received; command denied")
		output.Flush()
		return false
	}

	switch strings.TrimSpace(choice) {
	case "1":
		return true
	case "2":
		kind, rule := ParseTrustRule(executablePath)
		fmt.Fprintf(output, "Trust rule %q for this session? [y/N]: ", rule)
		output.Flush()

		var confirmation string
		if _, err := fmt.Fscanln(os.Stdin, &confirmation); err != nil ||
			(!strings.EqualFold(strings.TrimSpace(confirmation), "y") && !strings.EqualFold(strings.TrimSpace(confirmation), "yes")) {
			fmt.Fprintln(output, "session trust declined; command denied")
			output.Flush()
			return false
		}

		entry := TrustEntry{Rule: rule, Kind: kind, Source: SourceInteractive}
		state.trustStore.Add(entry)
		return true
	default:
		fmt.Fprintln(output, "command denied")
		output.Flush()
		return false
	}
}
