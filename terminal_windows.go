//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	ENABLE_LINE_INPUT                  = 0x0002
	ENABLE_ECHO_INPUT                  = 0x0004
	ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
)

const (
	is_windows = true
)

func RunWinCommands(cmdArgs []string) bool {
	if cmdArgs[0] == "mkdir" {
		if len(cmdArgs) > 1 {
			err := os.Mkdir(cmdArgs[1], os.ModeDir)
			if err != nil {
				fmt.Printf("%s%v%s", Red, err, Reset)
			}
		}
		return true
	}
	if cmdArgs[0] == "clear" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			fmt.Printf("%s%v%s", Red, err, Reset)
		}
		return true
	}
	if cmdArgs[0] == "echo" {
		if len(cmdArgs) > 1 {
			for _, str := range cmdArgs[1:] {
				fmt.Print(str, " ")
			}
			fmt.Println()
		}
		return true
	}
	if cmdArgs[0] == "cp" {
		if len(cmdArgs) > 2 {
			err := copyDir(cmdArgs[1], cmdArgs[2])
			if err != nil {
				fmt.Printf("%s%v%s", Red, err, Reset)
			}
		}
		return true
	}
	if cmdArgs[0] == "mv" {
		if len(cmdArgs) > 2 {
			err := os.Rename(cmdArgs[1], cmdArgs[2])
			if err != nil {
				fmt.Printf("%s%v%s", Red, err, Reset)
			}
		}
		return true
	}
	if cmdArgs[0] == "rm" {
		if len(cmdArgs) > 1 {
			err := os.RemoveAll(cmdArgs[1])
			if err != nil {
				fmt.Printf("%s%v%s", Red, err, Reset)
			}
		}
		return true
	}

	return false
}

func copyDir(src, dst string) error {
	// get properties of source dir
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// create destination dir
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	// get contents of the source dir
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	// copy each file/dir in the source dir to destination dir
	for _, entry := range entries {
		srcPath := src + "/" + entry.Name()
		dstPath := dst + "/" + entry.Name()

		// recursively copy a directory
		if entry.IsDir() {
			err = copyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			// perform copy operation on a file
			err = copyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err == nil {
		sourceInfo, err := os.Stat(src)
		if err != nil {
			err = os.Chmod(dst, sourceInfo.Mode())
		}

	}
	return err
}

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

func InitTerminal() {
	handle := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	procGetConsoleMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	mode |= ENABLE_VIRTUAL_TERMINAL_PROCESSING
	procSetConsoleMode.Call(uintptr(handle), uintptr(mode))
}

type TerminalState struct {
	mode uint32
}

func MakeRaw(fd uintptr) (*TerminalState, error) {
	var mode uint32
	ret, _, err := procGetConsoleMode.Call(fd, uintptr(unsafe.Pointer(&mode)))
	if ret == 0 {
		return nil, err
	}

	raw := mode &^ (uint32(ENABLE_ECHO_INPUT) | uint32(ENABLE_LINE_INPUT))

	ret, _, err = procSetConsoleMode.Call(fd, uintptr(raw))
	if ret == 0 {
		return nil, err
	}

	return &TerminalState{mode: mode}, nil
}

func RestoreTerminal(fd uintptr, state *TerminalState) {
	procSetConsoleMode.Call(fd, uintptr(state.mode))
}
