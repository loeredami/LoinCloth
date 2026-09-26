package main

import "testing"

func TestEvaluateTrustAllowsDefaultCloth(t *testing.T) {
	decision := EvaluateTrust(TrustStore{}, "/usr/bin/example", SourceDefaultCloth, false)
	if decision != TrustAllow {
		t.Fatalf("default.cloth decision: got %s, want allow", decision)
	}
}

func TestEvaluateTrustAllowsTrustedExecutable(t *testing.T) {
	kind, rule := ParseTrustRule("/usr/bin/example")
	store := TrustStore{}
	store.Add(TrustEntry{Rule: rule, Kind: kind, Source: SourceInteractive})

	decision := EvaluateTrust(store, "/usr/bin/example", SourceNonInteractive, false)
	if decision != TrustAllow {
		t.Fatalf("trusted executable decision: got %s, want allow", decision)
	}
}

func TestEvaluateTrustPromptsInteractiveUntrustedCommand(t *testing.T) {
	decision := EvaluateTrust(TrustStore{}, "/usr/bin/example", SourceInteractive, true)
	if decision != TrustPrompt {
		t.Fatalf("interactive decision: got %s, want prompt", decision)
	}
}

func TestEvaluateTrustPromptsForGrayListedSourceEvenWhenExecutableTrusted(t *testing.T) {
	kind, rule := ParseTrustRule("/usr/bin/example")
	store := TrustStore{}
	store.Add(TrustEntry{Rule: rule, Kind: kind, Source: SourceInteractive})

	decision := EvaluateTrust(store, "/usr/bin/example", SourceClothFile, true)
	if decision != TrustPrompt {
		t.Fatalf("gray-listed source decision: got %s, want prompt", decision)
	}
}

func TestEvaluateTrustDeniesNonInteractiveGrayListedSource(t *testing.T) {
	kind, rule := ParseTrustRule("/usr/bin/example")
	store := TrustStore{}
	store.Add(TrustEntry{Rule: rule, Kind: kind, Source: SourceInteractive})

	decision := EvaluateTrust(store, "/usr/bin/example", SourceClothFile, false)
	if decision != TrustDeny {
		t.Fatalf("non-interactive gray-listed decision: got %s, want deny", decision)
	}
}

func TestEvaluateTrustDeniesNonInteractiveUntrustedCommand(t *testing.T) {
	decision := EvaluateTrust(TrustStore{}, "/usr/bin/example", SourceNonInteractive, false)
	if decision != TrustDeny {
		t.Fatalf("non-interactive decision: got %s, want deny", decision)
	}
}
