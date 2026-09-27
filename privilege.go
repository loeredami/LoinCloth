package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

const adminSessionFlag = "--loin-admin-session"

type commandExitError struct {
	code int
}

func (err commandExitError) Error() string {
	return fmt.Sprintf("elevated command exited with status %d", err.code)
}

func (err commandExitError) ExitCode() int {
	return err.code
}

func resolveCommandExecutable(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("empty command")
	}
	if args[0] == "sudo" {
		target, err := sudoTarget(args)
		if err != nil {
			return "", err
		}
		return exec.LookPath(target[0])
	}
	return exec.LookPath(args[0])
}

func commandResolutionFailure(args []string, output io.Writer, state *State) int {
	if len(args) > 0 && args[0] == "sudo" {
		target, err := sudoTarget(args)
		if err != nil {
			fmt.Fprintf(output, "%s%v%s\n", state.GetColor(state.config.ErrorCol), err, state.Reset())
			return 2
		}
		fmt.Fprintf(output, "%sCommand not found: %s%s\n", state.GetColor(state.config.ErrorCol), target[0], state.Reset())
		return 127
	}
	fmt.Fprintf(output, "%sCommand not found: %s%s\n", state.GetColor(state.config.ErrorCol), args[0], state.Reset())
	return 127
}

func handleAdminModeCommand(state *State, commands []PipelineCommand, source CommandSource, output io.Writer) (bool, int) {
	containsAdminCommand := false
	for _, command := range commands {
		if len(command.args) > 0 && (command.args[0] == "!enter-admin" || command.args[0] == "!exit-admin") {
			containsAdminCommand = true
		}
	}
	if !containsAdminCommand {
		return false, 0
	}
	if source != SourceInteractive || state == nil || !state.interactiveInput {
		fmt.Fprintln(output, "!enter-admin and !exit-admin require direct interactive input")
		return true, 1
	}
	if len(commands) != 1 || len(commands[0].args) != 1 || commands[0].stdinPath != "" || commands[0].stdoutPath != "" {
		fmt.Fprintln(output, "!enter-admin and !exit-admin must be entered alone, without a pipeline or redirection")
		return true, 2
	}

	switch commands[0].args[0] {
	case "!enter-admin":
		if state.administratorMode || isAdministrator() {
			fmt.Fprintln(output, "administrator mode is already active")
			return true, 1
		}
		if err := enterAdminSession(output, os.Stderr); err != nil {
			fmt.Fprintf(output, "Error: %v\n", err)
			return true, processExitStatus(err)
		}
	case "!exit-admin":
		if !state.administratorMode {
			fmt.Fprintln(output, "administrator mode is not active")
			return true, 1
		}
		state.exitAdminMode = true
	}
	return true, 0
}

func sudoTarget(args []string) ([]string, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("sudo requires a command")
	}
	target := args[1:]
	if target[0] == "--" {
		target = target[1:]
	}
	if len(target) == 0 {
		return nil, fmt.Errorf("sudo requires a command after --")
	}
	if target[0] == "-" || (len(target[0]) > 1 && target[0][0] == '-') {
		return nil, fmt.Errorf("sudo options are not supported; use sudo <command> [arguments]")
	}
	return target, nil
}

func explicitPrivilegeRequestAllowed(state *State) bool {
	return state != nil && state.commandSource == SourceInteractive && state.interactiveInput
}
