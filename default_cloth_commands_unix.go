//go:build linux || darwin

package main

func generatedDefaultClothCommands() []string {
	return []string{
		"!trust /usr/bin/echo",
		"!trust /usr/bin/cat",
		"!trust /usr/bin/git",
		"!trust /usr/bin/grep",
		"!trust /usr/bin/find",
		"!trust /usr/bin/curl",
		"!trust /usr/bin/mkdir",
		"!trust /usr/bin/touch",
	}
}
