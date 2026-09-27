package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
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

type trustStoreFile struct {
	Version int          `json:"version"`
	Entries []TrustEntry `json:"entries"`
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

func (store TrustStore) Clone() TrustStore {
	return TrustStore{entries: store.Entries()}
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

func DefaultTrustStorePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, ".loin", "trust.json"), nil
}

func LoadTrustStore(path string) (TrustStore, error) {
	store, _, _, err := loadTrustStoreSnapshot(path)
	return store, err
}

func loadTrustStoreSnapshot(path string) (TrustStore, [32]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return TrustStore{}, [32]byte{}, false, nil
	}
	if err != nil {
		return TrustStore{}, [32]byte{}, false, err
	}

	store, err := parseTrustStore(data)
	if err != nil {
		return TrustStore{}, [32]byte{}, false, err
	}
	return store, sha256.Sum256(data), true, nil
}

func parseTrustStore(data []byte) (TrustStore, error) {
	var disk trustStoreFile
	if err := json.Unmarshal(data, &disk); err != nil {
		return TrustStore{}, fmt.Errorf("invalid trust store: %w", err)
	}
	if disk.Version != 1 {
		return TrustStore{}, fmt.Errorf("unsupported trust store version %d", disk.Version)
	}

	store := TrustStore{}
	for _, entry := range disk.Entries {
		if err := validateTrustEntry(entry); err != nil {
			return TrustStore{}, fmt.Errorf("invalid trust store: %w", err)
		}
		store.Add(entry)
	}
	return store, nil
}

func trustStoreSnapshot(path string) ([32]byte, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return [32]byte{}, false, nil
	}
	if err != nil {
		return [32]byte{}, false, err
	}
	return sha256.Sum256(data), true, nil
}

func SaveTrustStore(path string, store TrustStore) error {
	for _, entry := range store.Entries() {
		if err := validateTrustEntry(entry); err != nil {
			return fmt.Errorf("cannot save invalid trust entry: %w", err)
		}
	}

	data, err := json.MarshalIndent(trustStoreFile{
		Version: 1,
		Entries: store.Entries(),
	}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".trust-store-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
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
