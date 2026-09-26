package main

import "testing"

func TestConfigurationCommandSource(t *testing.T) {
	tests := []struct {
		name      string
		isDefault bool
		protected bool
		want      CommandSource
	}{
		{name: "validated default", isDefault: true, protected: true, want: SourceDefaultCloth},
		{name: "unvalidated default is gray-listed", isDefault: true, protected: false, want: SourceClothFile},
		{name: "explicit development file", isDefault: false, protected: true, want: SourceDevelopmentCloth},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := configurationCommandSource(test.isDefault, test.protected); got != test.want {
				t.Fatalf("source = %s, want %s", got, test.want)
			}
		})
	}
}
