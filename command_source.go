package main

type CommandSource int

const (
	SourceInteractive CommandSource = iota
	SourceDefaultCloth
	SourceClothFile
	SourceDevelopmentCloth
)

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
	default:
		return "unknown"
	}
}
