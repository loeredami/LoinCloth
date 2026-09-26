//go:build windows

package main

import (
	"fmt"
	"os"
)

func readProtectedDefaultCloth(path string) ([]byte, error) {
	return nil, fmt.Errorf("safe default.cloth ACL validation is not implemented on Windows")
}

func readDefaultCloth(path string) ([]byte, error) {
	return os.ReadFile(path)
}
