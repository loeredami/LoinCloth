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

	if err := trustStoreAvailable(state); err != nil {
		fmt.Fprintf(writer, "%v\n", err)
		writer.Flush()
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
	fmt.Fprintln(output, "2. Add command to allow list")
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
		if err := trustStoreAvailable(state); err != nil {
			fmt.Fprintf(output, "%v\n", err)
			output.Flush()
			return false
		}
		kind, rule := ParseTrustRule(executablePath)
		fmt.Fprintf(output, "Persist trust rule %q? [y/N]: ", rule)
		output.Flush()

		var confirmation string
		if _, err := fmt.Fscanln(os.Stdin, &confirmation); err != nil ||
			(!strings.EqualFold(strings.TrimSpace(confirmation), "y") && !strings.EqualFold(strings.TrimSpace(confirmation), "yes")) {
			fmt.Fprintln(output, "trust persistence declined; command denied")
			output.Flush()
			return false
		}

		entry := TrustEntry{Rule: rule, Kind: kind, Source: SourceInteractive}
		candidate := TrustStore{}
		for _, existing := range state.trustStore.Entries() {
			candidate.Add(existing)
		}
		candidate.Add(entry)
		if state.trustStorePath == "" {
			path, err := DefaultTrustStorePath()
			if err != nil {
				fmt.Fprintf(output, "could not locate trust store: %v\n", err)
				output.Flush()
				return false
			}
			state.trustStorePath = path
		}
		if err := trustStoreAvailable(state); err != nil {
			fmt.Fprintf(output, "%v\n", err)
			output.Flush()
			return false
		}
		if err := SaveTrustStore(state.trustStorePath, candidate); err != nil {
			fmt.Fprintf(output, "could not save trust entry: %v\n", err)
			output.Flush()
			return false
		}
		if err := recordTrustStoreSnapshot(state); err != nil {
			fmt.Fprintf(output, "%v\n", err)
			output.Flush()
			return false
		}
		state.trustStore = candidate
		return true
	default:
		fmt.Fprintln(output, "command denied")
		output.Flush()
		return false
	}
}
