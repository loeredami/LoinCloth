//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// Define missing Windows Console constants
const (
	ENABLE_LINE_INPUT = 0x0002
	ENABLE_ECHO_INPUT = 0x0004
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

type TerminalState struct {
	mode uint32
}

func MakeRaw(fd uintptr) (*TerminalState, error) {
	var mode uint32
	// Call GetConsoleMode via the lazy DLL procedure
	ret, _, err := procGetConsoleMode.Call(fd, uintptr(unsafe.Pointer(&mode)))
	if ret == 0 {
		return nil, err
	}

	// Disable line input and echo
	raw := mode &^ (uint32(ENABLE_ECHO_INPUT) | uint32(ENABLE_LINE_INPUT))

	// Call SetConsoleMode
	ret, _, err = procSetConsoleMode.Call(fd, uintptr(raw))
	if ret == 0 {
		return nil, err
	}

	return &TerminalState{mode: mode}, nil
}

func RestoreTerminal(fd uintptr, state *TerminalState) {
	procSetConsoleMode.Call(fd, uintptr(state.mode))
}
