//go:build windows

package main

import (
	"os"
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
