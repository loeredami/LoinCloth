//go:build linux || darwin

package main

func generatedDefaultClothCommands() []string {
	return []string{
		"!trust-bootstrap whoami",
		"!trust-bootstrap id",
		"!trust-bootstrap uname",
		"!trust-bootstrap uptime",
		"!trust-bootstrap pwd",
	}
}
