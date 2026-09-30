package main

import "strings"

// LaunchClass separates workspace commands, shell built-ins, and external
// executables. Only external executables use the session trust list.
type LaunchClass int

const (
	LaunchWorkspace LaunchClass = iota
	LaunchBuiltin
	LaunchExternal
)

func (class LaunchClass) String() string {
	switch class {
	case LaunchWorkspace:
		return "workspace"
	case LaunchBuiltin:
		return "builtin"
	case LaunchExternal:
		return "external"
	default:
		return "unknown"
	}
}

// classifyLaunch decides which authorization path a command name uses.
// Workspace `!` commands and shell built-ins never consult the executable trust list.
func classifyLaunch(name string) LaunchClass {
	if name == "" {
		return LaunchExternal
	}
	if strings.HasPrefix(name, "!") {
		return LaunchWorkspace
	}
	if isPipelineInternal(name) {
		return LaunchBuiltin
	}
	return LaunchExternal
}

func requiresExecutableTrust(name string) bool {
	return classifyLaunch(name) == LaunchExternal
}
