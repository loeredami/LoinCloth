package main

import "testing"

func TestClassifyLaunchSeparatesWorkspaceBuiltinAndExternal(t *testing.T) {
	cases := []struct {
		name string
		want LaunchClass
	}{
		{name: "!wear", want: LaunchWorkspace},
		{name: "!trust", want: LaunchWorkspace},
		{name: "cd", want: LaunchBuiltin},
		{name: "ls", want: LaunchBuiltin},
		{name: "date", want: LaunchExternal},
		{name: "/usr/bin/printf", want: LaunchExternal},
	}
	for _, test := range cases {
		if got := classifyLaunch(test.name); got != test.want {
			t.Fatalf("classifyLaunch(%q) = %s, want %s", test.name, got, test.want)
		}
		if requiresExecutableTrust(test.name) != (test.want == LaunchExternal) {
			t.Fatalf("requiresExecutableTrust(%q) mismatch for class %s", test.name, test.want)
		}
	}
}
