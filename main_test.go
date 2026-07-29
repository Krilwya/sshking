package main

import "testing"

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
}
