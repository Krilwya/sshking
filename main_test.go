package main

import (
	"strings"
	"testing"
)

func TestApplicationOptionsUseNativeMacWindowChrome(t *testing.T) {
	options := applicationOptions(&App{}, "darwin")

	if options.Frameless {
		t.Fatal("macOS must keep the native Cocoa frame")
	}
	if options.Mac == nil || options.Mac.TitleBar == nil {
		t.Fatal("macOS hidden-inset title bar must be configured")
	}
	if !options.Mac.TitleBar.FullSizeContent {
		t.Fatal("macOS content must extend into the native title bar")
	}
	if options.Mac.TitleBar.HideTitleBar {
		t.Fatal("hiding the native title bar also removes the traffic lights")
	}
	if options.Mac.About == nil {
		t.Fatal("macOS must expose native application information")
	}
}

func TestApplicationOptionsKeepCustomWindowsChrome(t *testing.T) {
	options := applicationOptions(&App{}, "windows")

	if !options.Frameless {
		t.Fatal("Windows must keep the custom application title bar")
	}
	if options.Windows == nil {
		t.Fatal("Windows options must be configured")
	}
	if options.Windows.DisableFramelessWindowDecorations {
		t.Fatal("DWM decorations are required for native shadow and rounded corners")
	}
	if options.Windows.WebviewGpuIsDisabled {
		t.Fatal("Windows WebView GPU must remain enabled for terminal rendering")
	}
}

func TestValidTmuxSessionName(t *testing.T) {
	for _, name := range []string{"sshking", "sshking-prod_2", "A1"} {
		if !validTmuxSessionName(name) {
			t.Fatalf("expected valid tmux session name: %q", name)
		}
	}
	for _, name := range []string{"", "bad.name", "bad:name", "bad name", "$(command)"} {
		if validTmuxSessionName(name) {
			t.Fatalf("expected invalid tmux session name: %q", name)
		}
	}
}

func TestTmuxSessionIsScopedToStableTab(t *testing.T) {
	first := tmuxSessionForTab("sshking", "tab-one")
	if first != tmuxSessionForTab("sshking", "tab-one") {
		t.Fatal("the same tab must retain its tmux session name")
	}
	if first == tmuxSessionForTab("sshking", "tab-two") {
		t.Fatal("different tabs must use different tmux sessions")
	}
	if !strings.HasPrefix(first, "sshking-") || len(first) > 64 {
		t.Fatalf("unexpected scoped tmux name: %q", first)
	}
}
