//go:build windows

package main

func generatedDefaultClothCommands() []string {
	return []string{
		"!trust whoami.exe",
		"!trust hostname.exe",
		"!trust tasklist.exe",
		"!trust systeminfo.exe",
		"!trust where.exe",
	}
}
