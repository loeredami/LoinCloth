package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type TrustMatchKind int

const (
	TrustExactPath TrustMatchKind = iota
	TrustBasename
	TrustGlob
)

type TrustEntry struct {
	Rule   string
	Kind   TrustMatchKind
	Source CommandSource
}

type TrustStore struct {
	entries []TrustEntry
}

func isWorkspaceTrustRule(rule string) bool {
	return strings.HasPrefix(strings.TrimSpace(rule), "!")
}

func validateTrustEntry(entry TrustEntry) error {
	if strings.TrimSpace(entry.Rule) == "" {
		return fmt.Errorf("trust entry contains an empty rule")
	}
	if isWorkspaceTrustRule(entry.Rule) {
		return fmt.Errorf("workspace commands cannot be trusted as executables")
	}
	if entry.Kind < TrustExactPath || entry.Kind > TrustGlob {
		return fmt.Errorf("trust entry contains an invalid rule kind %d", entry.Kind)
	}
	return nil
}

func normalizeExecutablePath(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	return filepath.Clean(path)
}

func executableBaseName(path string) string {
	path = strings.TrimRight(path, "/\\")
	if separator := strings.LastIndexAny(path, "/\\"); separator >= 0 {
		return path[separator+1:]
	}
	return path
}

func (entry TrustEntry) Matches(executablePath string) bool {
	if entry.Rule == "" || isWorkspaceTrustRule(entry.Rule) || executablePath == "" {
		return false
	}

	switch entry.Kind {
	case TrustExactPath:
		return normalizeExecutablePath(entry.Rule) == normalizeExecutablePath(executablePath)
	case TrustBasename:
		return executableBaseName(executablePath) == entry.Rule
	case TrustGlob:
		matched, err := filepath.Match(entry.Rule, executableBaseName(executablePath))
		return err == nil && matched
	default:
		return false
	}
}

func (store *TrustStore) Add(entry TrustEntry) {
	if validateTrustEntry(entry) != nil {
		return
	}
	for _, existing := range store.entries {
		if existing == entry {
			return
		}
	}
	store.entries = append(store.entries, entry)
}

func (store TrustStore) Allows(executablePath string) bool {
	for _, entry := range store.entries {
		if entry.Matches(executablePath) {
			return true
		}
	}
	return false
}

func (store TrustStore) Entries() []TrustEntry {
	return append([]TrustEntry(nil), store.entries...)
}

func (store *TrustStore) Remove(entry TrustEntry) bool {
	for i, existing := range store.entries {
		if existing == entry {
			store.entries = append(store.entries[:i], store.entries[i+1:]...)
			return true
		}
	}
	return false
}

func (store *TrustStore) RemoveRule(kind TrustMatchKind, rule string) bool {
	for i, existing := range store.entries {
		if existing.Kind == kind && existing.Rule == rule {
			store.entries = append(store.entries[:i], store.entries[i+1:]...)
			return true
		}
	}
	return false
}

func ParseTrustRule(rule string) (TrustMatchKind, string) {
	rule = strings.TrimSpace(rule)
	if strings.ContainsAny(rule, "*?[") {
		return TrustGlob, rule
	}
	if strings.ContainsAny(rule, `/\`) {
		return TrustExactPath, normalizeExecutablePath(rule)
	}
	return TrustBasename, rule
}
