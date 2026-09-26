package main

type CommandSource int

const (
	SourceInteractive CommandSource = iota
	SourceDefaultCloth
	SourceClothFile
	SourceDevelopmentCloth
	SourceNonInteractive
)

func configurationCommandSource(isDefault, protected bool) CommandSource {
	if !isDefault {
		return SourceDevelopmentCloth
	}
	if protected {
		return SourceDefaultCloth
	}
	return SourceClothFile
}

func (source CommandSource) String() string {
	switch source {
	case SourceInteractive:
		return "interactive"
	case SourceDefaultCloth:
		return "default.cloth"
	case SourceClothFile:
		return ".cloth"
	case SourceDevelopmentCloth:
		return "development .cloth"
	case SourceNonInteractive:
		return "non-interactive input"
	default:
		return "unknown"
	}
}
