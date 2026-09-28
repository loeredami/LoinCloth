//go:build linux || darwin

package main

func generatedDefaultClothCommands() []string {
	return []string{
		"!trust whoami",
		"!trust id",
		"!trust uname",
		"!trust uptime",
		"!trust pwd",
	}
}
