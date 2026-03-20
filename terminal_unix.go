//go:build darwin || linux

package main

import (
	"syscall"
	"unsafe"
)

type TerminalState struct {
	termios syscall.Termios
}

func MakeRaw(fd uintptr) (*TerminalState, error) {
	var old syscall.Termios
	// TCGETS constant for Linux (0x5401)
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, 0x5401, uintptr(unsafe.Pointer(&old)), 0, 0, 0); err != 0 {
		return nil, err
	}

	raw := old
	// Disable Echo, Line buffering (ICANON), and Interrupt signals
	raw.Lflag &^= (syscall.ECHO | syscall.ICANON | syscall.ISIG)

	// TCSETS constant (0x5402)
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, 0x5402, uintptr(unsafe.Pointer(&raw)), 0, 0, 0); err != 0 {
		return nil, err
	}

	return &TerminalState{termios: old}, nil
}

func RestoreTerminal(fd uintptr, state *TerminalState) {
	syscall.Syscall6(syscall.SYS_IOCTL, fd, 0x5402, uintptr(unsafe.Pointer(&state.termios)), 0, 0, 0)
}
