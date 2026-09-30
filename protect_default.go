package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"compress/gzip"

	"github.com/loeredami/ungo"
)

var config_backup bytes.Buffer

func PackageConfigurationToMemory(state *State) ungo.Optional[error] {
	data, err := os.ReadFile(state.GetConfigFilePath())
	if err != nil {
		return ungo.Some[error](err)
	}

	config_backup.Reset()
	gz := gzip.NewWriter(&config_backup)
	if _, err := gz.Write(data); err != nil {
		return ungo.Some[error](err)
	}
	if err := gz.Close(); err != nil {
		return ungo.Some[error](err)
	}

	return ungo.None[error]()
}

func RestoreConfiguration(state *State) ungo.Optional[error] {
	reader := bytes.NewReader(config_backup.Bytes())
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return ungo.Some[error](err)
	}

	config_data, err := io.ReadAll(gz)
	if err != nil {
		return ungo.Some[error](err)
	}

	err = os.WriteFile(state.GetConfigFilePath(), config_data, 0644)
	if err != nil {
		return ungo.Some[error](err)
	}

	return ungo.None[error]()
}

func GetOldConfig(state *State) (string, error) {
	reader := bytes.NewReader(config_backup.Bytes())
	gz, err := gzip.NewReader(reader)
	if err != nil {
		return "", err
	}

	config_data, err := io.ReadAll(gz)
	if err != nil {
		return "", err
	}

	return string(config_data), nil
}

func PrintGitStyleDiff(old, new string) {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	m, n := len(oldLines), len(newLines)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			if oldLines[i] == newLines[j] {
				dp[i+1][j+1] = dp[i][j] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i+1][j+1] = dp[i+1][j]
			} else {
				dp[i+1][j+1] = dp[i][j+1]
			}
		}
	}

	type diffLine struct {
		kind rune
		text string
	}

	var diff []diffLine
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			diff = append(diff, diffLine{' ', oldLines[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			diff = append(diff, diffLine{'+', newLines[j-1]})
			j--
		} else if i > 0 && (j == 0 || dp[i][j-1] < dp[i-1][j]) {
			diff = append(diff, diffLine{'-', oldLines[i-1]})
			i--
		}
	}

	for k := 0; k < len(diff)/2; k++ {
		diff[k], diff[len(diff)-1-k] = diff[len(diff)-1-k], diff[k]
	}

	const contextRadius = 2
	show := make([]bool, len(diff))
	for idx, line := range diff {
		if line.kind != ' ' {
			start := max(0, idx-contextRadius)
			end := min(len(diff)-1, idx+contextRadius)
			for c := start; c <= end; c++ {
				show[c] = true
			}
		}
	}

	inSkippedBlock := false
	for idx, line := range diff {
		if !show[idx] {
			if !inSkippedBlock {
				fmt.Println("\033[36m@@ ... @@\033[0m")
				inSkippedBlock = true
			}
			continue
		}
		inSkippedBlock = false

		switch line.kind {
		case '-':
			fmt.Println(Red + "-" + line.text + "\033[0m")
		case '+':
			fmt.Println(Green + "+" + line.text + "\033[0m")
		case ' ':
			fmt.Println(" " + line.text)
		}
	}
}

func HasConfigChanged(state *State) bool {
	old, err := GetOldConfig(state)
	if err != nil {
		return false
	}
	new, err := os.ReadFile(state.GetConfigFilePath())
	if err != nil {
		return false
	}
	return old != string(new)
}

func PromptKeepConfiguration(state *State) ungo.Optional[error] {
	fmt.Println("")
	old, err := GetOldConfig(state)
	if err != nil {
		return ungo.Some[error](err)
	}
	new, err := os.ReadFile(state.GetConfigFilePath())
	if err != nil {
		return ungo.Some[error](err)
	}

	PrintGitStyleDiff(old, string(new))

	fmt.Println("Your configuration file has changed. Keep the changes? [y/n]")
	fmt.Print("")

	return ungo.None[error]()
}
