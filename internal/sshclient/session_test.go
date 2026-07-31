package sshclient

import (
	"bytes"
	"strings"
	"testing"

	"sshking/internal/config"
)

type writeCloserBuffer struct {
	bytes.Buffer
}

func TestInteractiveCommandUsesPersistentTmuxSession(t *testing.T) {
	command := interactiveCommand(config.Server{Shell: "zsh", UseTmux: true, TmuxSession: "sshking-prod"})
	for _, fragment := range []string{"command -v tmux", "has-session -t 'sshking-prod'", "attach-session -t 'sshking-prod'", "new-session -d -s 'sshking-prod'", "set-option -t 'sshking-prod' mouse on", `'exec zsh -l'`} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("interactive command missing %q: %s", fragment, command)
		}
	}
}

func TestInteractiveCommandFallsBackWithoutTmux(t *testing.T) {
	command := interactiveCommand(config.Server{Shell: "bash", UseTmux: true})
	if !strings.Contains(command, "tmux is not installed") || !strings.HasSuffix(command, "exec bash -l; fi") {
		t.Fatalf("unexpected tmux fallback: %s", command)
	}
}

func (writeCloserBuffer) Close() error { return nil }

func TestShellCommand(t *testing.T) {
	tests := map[string]string{
		"":        "",
		"default": "",
		"zsh":     "exec zsh -l",
		"bash":    "exec bash -l",
		"fish":    "exec fish -l",
	}
	for input, want := range tests {
		if got := shellCommand(input); got != want {
			t.Fatalf("shellCommand(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSendInputPreservesTerminalControlBytes(t *testing.T) {
	stdin := &writeCloserBuffer{}
	session := &Session{stdin: stdin}
	input := "ping\x03\x1b[A\t\r"
	if err := session.SendInput(input); err != nil {
		t.Fatal(err)
	}
	if got := stdin.String(); got != input {
		t.Fatalf("SendInput changed terminal bytes: got %q, want %q", got, input)
	}
}
