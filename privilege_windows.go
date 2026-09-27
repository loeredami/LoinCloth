//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

const (
	tokenQuery                = 0x0008
	tokenElevationClass       = 20
	seeMaskNoAsync            = 0x00000100
	seeMaskNoCloseProc        = 0x00000040
	waitObject0               = 0
	waitFailed                = 0xFFFFFFFF
	errorCancelled            = 1223
	supportsElevatedPipelines = false
)

type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr
	hProcess       syscall.Handle
}

var (
	privilegeShell32 = syscall.NewLazyDLL("shell32.dll")
	shellExecuteExW  = privilegeShell32.NewProc("ShellExecuteExW")
	privilegeKernel  = syscall.NewLazyDLL("kernel32.dll")
	waitForSingle    = privilegeKernel.NewProc("WaitForSingleObject")
	getExitCode      = privilegeKernel.NewProc("GetExitCodeProcess")
	closeHandle      = privilegeKernel.NewProc("CloseHandle")
	privilegeAdvapi  = syscall.NewLazyDLL("advapi32.dll")
	openProcessToken = privilegeAdvapi.NewProc("OpenProcessToken")
	getTokenInfo     = privilegeAdvapi.NewProc("GetTokenInformation")
	getCurrentProc   = privilegeKernel.NewProc("GetCurrentProcess")
)

func isAdministrator() bool {
	process, _, _ := getCurrentProc.Call()
	var token syscall.Handle
	ok, _, _ := openProcessToken.Call(process, tokenQuery, uintptr(unsafe.Pointer(&token)))
	if ok == 0 {
		return false
	}
	defer closeHandle.Call(uintptr(token))

	var elevation uint32
	var returned uint32
	ok, _, _ = getTokenInfo.Call(uintptr(token), tokenElevationClass, uintptr(unsafe.Pointer(&elevation)), unsafe.Sizeof(elevation), uintptr(unsafe.Pointer(&returned)))
	return ok != 0 && returned >= uint32(unsafe.Sizeof(elevation)) && elevation != 0
}

func runSudoCommand(target []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if !isStandardStream(stdin, os.Stdin) || !isStandardStream(stdout, os.Stdout) || !isStandardStream(stderr, os.Stderr) {
		return fmt.Errorf("elevated Windows commands currently require the interactive console and do not support pipelines or redirection")
	}
	if len(target) == 0 {
		return fmt.Errorf("sudo requires a command")
	}
	path, err := exec.LookPath(target[0])
	if err != nil {
		return fmt.Errorf("find elevated command %q: %w", target[0], err)
	}
	return shellExecuteRunAs(path, target[1:])
}

func enterAdminSession(stdout, stderr io.Writer) error {
	if !isStandardStream(stdout, os.Stdout) || !isStandardStream(stderr, os.Stderr) {
		return fmt.Errorf("administrator mode requires an interactive console")
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate LoinCloth executable: %w", err)
	}
	return shellExecuteRunAs(executable, []string{adminSessionFlag})
}

func isStandardStream(stream any, expected *os.File) bool {
	file, ok := stream.(*os.File)
	return ok && file == expected
}

func shellExecuteRunAs(path string, args []string) error {
	verb, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	directory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}
	directoryPtr, err := syscall.UTF16PtrFromString(directory)
	if err != nil {
		return err
	}
	var parameters *uint16
	if len(args) > 0 {
		quoted := make([]string, len(args))
		for i, arg := range args {
			quoted[i] = quoteWindowsArgument(arg)
		}
		parameters, err = syscall.UTF16PtrFromString(strings.Join(quoted, " "))
		if err != nil {
			return err
		}
	}

	info := shellExecuteInfo{
		cbSize:       uint32(unsafe.Sizeof(shellExecuteInfo{})),
		fMask:        seeMaskNoAsync | seeMaskNoCloseProc,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: parameters,
		lpDirectory:  directoryPtr,
		nShow:        1,
	}
	ok, _, callErr := shellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errorCancelled {
			return fmt.Errorf("administrator approval was cancelled")
		}
		if callErr != nil {
			return fmt.Errorf("request administrator approval: %w", callErr)
		}
		return fmt.Errorf("request administrator approval failed")
	}
	if info.hProcess == 0 {
		return fmt.Errorf("UAC did not return a process handle")
	}
	defer closeHandle.Call(uintptr(info.hProcess))

	waitResult, _, waitErr := waitForSingle.Call(uintptr(info.hProcess), ^uintptr(0))
	if waitResult == waitFailed || waitResult != waitObject0 {
		if waitErr != nil {
			return fmt.Errorf("wait for elevated process: %w", waitErr)
		}
		return fmt.Errorf("wait for elevated process returned status 0x%x", waitResult)
	}
	var code uint32
	ok, _, exitErr := getExitCode.Call(uintptr(info.hProcess), uintptr(unsafe.Pointer(&code)))
	if ok == 0 {
		return fmt.Errorf("read elevated process exit status: %w", exitErr)
	}
	if code != 0 {
		return commandExitError{code: int(code)}
	}
	return nil
}

func quoteWindowsArgument(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \t\n\v\"") {
		return arg
	}
	var quoted strings.Builder
	quoted.WriteByte('"')
	backslashes := 0
	for _, r := range arg {
		switch r {
		case '\\':
			backslashes++
		case '"':
			quoted.WriteString(strings.Repeat("\\", backslashes*2+1))
			quoted.WriteRune(r)
			backslashes = 0
		default:
			quoted.WriteString(strings.Repeat("\\", backslashes))
			quoted.WriteRune(r)
			backslashes = 0
		}
	}
	quoted.WriteString(strings.Repeat("\\", backslashes*2))
	quoted.WriteByte('"')
	return quoted.String()
}
