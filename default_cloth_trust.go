package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func generatedDefaultClothTrustTargets() []string {
	targets := make([]string, 0, len(generatedDefaultClothCommands()))
	for _, command := range generatedDefaultClothCommands() {
		fields := strings.Fields(command)
		if len(fields) != 2 || fields[0] != "!trust" {
			continue
		}
		targets = append(targets, fields[1])
	}
	return targets
}

func warnMissingDefaultClothTrustTargets(w *os.File) {
	for _, target := range generatedDefaultClothTrustTargets() {
		if _, err := exec.LookPath(target); err != nil {
			fmt.Fprintf(w, "Warning: default.cloth trust target %q is not available on PATH\n", target)
		}
	}
}
