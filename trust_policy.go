package main

type TrustDecision int

const (
	TrustAllow TrustDecision = iota
	TrustPrompt
	TrustDeny
)

func (decision TrustDecision) String() string {
	switch decision {
	case TrustAllow:
		return "allow"
	case TrustPrompt:
		return "prompt"
	case TrustDeny:
		return "deny"
	default:
		return "unknown"
	}
}

// EvaluateTrust applies the policy without prompting or launching a process.
// Prompting remains a separate concern so it can be tested independently from
// command execution and terminal state.
func EvaluateTrust(store TrustStore, executablePath string, source CommandSource, interactive bool) TrustDecision {
	if source == SourceDefaultCloth {
		return TrustAllow
	}
	if source == SourceClothFile || source == SourceDevelopmentCloth {
		if !interactive {
			return TrustDeny
		}
		return TrustPrompt
	}
	if store.Allows(executablePath) {
		return TrustAllow
	}
	if !interactive {
		return TrustDeny
	}
	return TrustPrompt
}
