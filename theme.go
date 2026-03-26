package main

const Reset = "\033[0m"
const Red = "\033[31m"
const Green = "\033[32m"
const Yellow = "\033[33m"
const Blue = "\033[34m"
const Magenta = "\033[35m"
const Cyan = "\033[36m"
const Gray = "\033[37m"
const White = "\033[97m"

type Theme struct {
	ErrorCol         string
	LSDirCol         string
	LSSymLinkCol     string
	LSExecCol        string
	LSNormalCol      string
	SudoPromptCol    string
	PromptCol        string
	IdxCol           string
	CurWSCol         string
	CurDirCol        string
	CurDirIndicCol   string
	GitBranchCol     string
	TimeCol          string
	TimePrefixCol    string
	ScopeCol         string
	InputCol         string
	PathCol          string
	InputStringCol   string
	InputNumCol      string
	InputPathCol     string
	InputVarCol      string
	InputBraceCol    string
	GhostCol         string
	WorkspaceNameCol string
}

type Configuration struct {
	Theme
	SudoPrompt string
	Prompt     string
	ScopeSign  string
	ColorMode  bool
}

func DefaultConfiguration() Configuration {
	return Configuration{
		Theme: Theme{
			ErrorCol:         Red,
			LSDirCol:         Yellow,
			LSSymLinkCol:     Cyan,
			LSExecCol:        Green,
			LSNormalCol:      White,
			SudoPromptCol:    Red,
			PromptCol:        Magenta,
			IdxCol:           Cyan,
			CurWSCol:         Yellow,
			CurDirCol:        Cyan,
			CurDirIndicCol:   Blue,
			GitBranchCol:     Green,
			TimeCol:          Yellow,
			TimePrefixCol:    Cyan,
			ScopeCol:         Yellow,
			InputCol:         Cyan,
			PathCol:          Blue,
			InputStringCol:   Green,
			InputNumCol:      Blue,
			InputPathCol:     Yellow,
			InputVarCol:      Magenta,
			InputBraceCol:    Blue,
			GhostCol:         Gray,
			WorkspaceNameCol: Yellow,
		},
		SudoPrompt: "#!",
		Prompt:     "»",
		ScopeSign:  ":",
		ColorMode:  true,
	}
}

func (s *State) GetColor(colorCode string) string {
	if !s.config.ColorMode {
		return ""
	}
	return colorCode
}

func (s *State) Reset() string {
	if !s.config.ColorMode {
		return ""
	}
	return Reset
}
