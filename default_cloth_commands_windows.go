//go:build windows

package main

func generatedDefaultClothCommands() []string {
	return []string{
		"!trust-bootstrap whoami.exe",
		"!trust-bootstrap hostname.exe",
		"!trust-bootstrap tasklist.exe",
		"!trust-bootstrap systeminfo.exe",
		"!trust-bootstrap where.exe",
	}
}
