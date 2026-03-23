//go:build darwin || linux

package main

import (
	"syscall"
	"unsafe"
)

func InitTerminal() {}

type TerminalState struct {
	termios syscall.Termios
}

func MakeRaw(fd uintptr) (*TerminalState, error) {
	var old syscall.Termios
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, 0x5401, uintptr(unsafe.Pointer(&old)), 0, 0, 0); err != 0 {
		return nil, err
	}

	raw := old
	raw.Lflag &^= (syscall.ECHO | syscall.ICANON | syscall.ISIG)

	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, 0x5402, uintptr(unsafe.Pointer(&raw)), 0, 0, 0); err != 0 {
		return nil, err
	}

	return &TerminalState{termios: old}, nil
}

func RestoreTerminal(fd uintptr, state *TerminalState) {
	syscall.Syscall6(syscall.SYS_IOCTL, fd, 0x5402, uintptr(unsafe.Pointer(&state.termios)), 0, 0, 0)
}
