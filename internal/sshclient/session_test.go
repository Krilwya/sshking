package sshclient

import (
	"bytes"
	"testing"
)

type writeCloserBuffer struct {
	bytes.Buffer
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
