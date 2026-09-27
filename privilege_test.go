package main

import (
	"bytes"
	"reflect"
	"testing"
)

func TestSudoTarget(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []string
		wantErr bool
	}{
		{name: "command and arguments", args: []string{"sudo", "echo", "hello world"}, want: []string{"echo", "hello world"}},
		{name: "explicit separator", args: []string{"sudo", "--", "echo", "hello"}, want: []string{"echo", "hello"}},
		{name: "missing command", args: []string{"sudo"}, wantErr: true},
		{name: "separator without command", args: []string{"sudo", "--"}, wantErr: true},
		{name: "unsupported sudo option", args: []string{"sudo", "-u", "root", "id"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := sudoTarget(test.args)
			if (err != nil) != test.wantErr {
				t.Fatalf("sudoTarget() error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && !reflect.DeepEqual(got, test.want) {
				t.Fatalf("sudoTarget() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestAdminModeCommandsRequireDirectInteractiveInput(t *testing.T) {
	state := &State{interactiveInput: true}
	command := PipelineCommand{args: []string{"!enter-admin"}}
	var output bytes.Buffer

	handled, status := handleAdminModeCommand(state, []PipelineCommand{command}, SourceClothFile, &output)
	if !handled || status == 0 {
		t.Fatalf("cloth command: handled=%v status=%d", handled, status)
	}
	if !bytes.Contains(output.Bytes(), []byte("direct interactive input")) {
		t.Fatalf("unexpected rejection message: %q", output.String())
	}

	output.Reset()
	state.commandSource = SourceInteractive
	handled, status = handleAdminModeCommand(state, []PipelineCommand{command, {args: []string{"echo", "no"}}}, SourceInteractive, &output)
	if !handled || status == 0 {
		t.Fatalf("pipeline command: handled=%v status=%d", handled, status)
	}
	if !bytes.Contains(output.Bytes(), []byte("must be entered alone")) {
		t.Fatalf("unexpected pipeline rejection message: %q", output.String())
	}
}

func TestSudoIsDeniedForClothAndNonInteractiveSources(t *testing.T) {
	for _, source := range []CommandSource{SourceDefaultCloth, SourceClothFile, SourceDevelopmentCloth, SourceNonInteractive} {
		t.Run(source.String(), func(t *testing.T) {
			state := newInputTestState()
			state.interactiveInput = true
			var output bytes.Buffer
			RunStringToSource(state, "sudo example", &output, source)
			if state.lastExitCode != 126 {
				t.Fatalf("exit status = %d, want 126; output=%q", state.lastExitCode, output.String())
			}
			if !bytes.Contains(output.Bytes(), []byte("requires direct interactive input")) {
				t.Fatalf("sudo was not denied as a non-interactive request: %q", output.String())
			}
		})
	}
}

func TestExitAdminModeRequestsChildShellExit(t *testing.T) {
	state := &State{interactiveInput: true, administratorMode: true}
	command := PipelineCommand{args: []string{"!exit-admin"}}

	handled, status := handleAdminModeCommand(state, []PipelineCommand{command}, SourceInteractive, &bytes.Buffer{})
	if !handled || status != 0 {
		t.Fatalf("exit command: handled=%v status=%d", handled, status)
	}
	if !state.exitAdminMode {
		t.Fatal("exit command did not request admin child shutdown")
	}
}

func TestAdministratorPromptCanBeLoadedButNotChangedDuringSession(t *testing.T) {
	state := newInputTestState()
	state.administratorMode = true
	state.loadingConfig = true

	if result := HandleStateCommands(state, []string{"!local", "sudo-prompt", "admin>"}); result.HasValue() {
		t.Fatalf("load configured admin prompt: %v", result.Value())
	}
	if state.config.SudoPrompt != "admin>" {
		t.Fatalf("loaded sudo prompt = %q, want admin>", state.config.SudoPrompt)
	}

	state.loadingConfig = false
	if result := HandleStateCommands(state, []string{"!local", "sudo-prompt", "changed>"}); !result.HasValue() {
		t.Fatal("interactive admin prompt change was accepted")
	}
	if state.config.SudoPrompt != "admin>" {
		t.Fatalf("rejected sudo prompt change modified config: %q", state.config.SudoPrompt)
	}
}

func TestSecurityStatusReportsAdminSessionState(t *testing.T) {
	state := newInputTestState()
	state.administratorMode = true
	var output bytes.Buffer
	if err := writeSecurityStatus(state, &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte("Privilege state: administrator")) {
		t.Fatalf("security status omitted admin mode: %q", output.String())
	}
}

func TestProcessExitStatusSupportsPlatformExitErrors(t *testing.T) {
	if got := processExitStatus(commandExitError{code: 17}); got != 17 {
		t.Fatalf("processExitStatus() = %d, want 17", got)
	}
}
