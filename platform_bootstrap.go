package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func platformBootstrapTargets() []string {
	targets := make([]string, 0, len(generatedDefaultClothCommands()))
	for _, command := range generatedDefaultClothCommands() {
		fields := strings.Fields(command)
		if len(fields) != 2 {
			continue
		}
		if fields[0] != "!trust-bootstrap" && fields[0] != "!trust" {
			continue
		}
		targets = append(targets, fields[1])
	}
	return targets
}

func isAllowedPlatformBootstrapTarget(target string) bool {
	kind, rule := ParseTrustRule(target)
	if kind != TrustBasename {
		return false
	}
	for _, allowed := range platformBootstrapTargets() {
		_, allowedRule := ParseTrustRule(allowed)
		if allowedRule == rule {
			return true
		}
	}
	return false
}

func warnMissingPlatformBootstrapTargets(w *os.File) {
	for _, target := range platformBootstrapTargets() {
		if _, err := exec.LookPath(target); err != nil {
			fmt.Fprintf(w, "Warning: platform bootstrap command %q is not available on PATH\n", target)
		}
	}
}
