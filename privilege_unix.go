//go:build darwin || linux

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

const supportsElevatedPipelines = true

func isAdministrator() bool {
	return os.Geteuid() == 0
}

func runSudoCommand(target []string, stdin io.Reader, stdout, stderr io.Writer) error {
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("sudo is unavailable: %w", err)
	}
	args := append([]string{"--"}, target...)
	command := exec.Command(sudoPath, args...)
	command.Stdin = stdin
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func enterAdminSession(stdout, stderr io.Writer) error {
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("sudo is unavailable: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate LoinCloth executable: %w", err)
	}
	command := exec.Command(sudoPath, "--", executable, adminSessionFlag)
	command.Stdin = os.Stdin
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}
