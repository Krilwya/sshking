package main

import (
	"embed"
	"log"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	application, err := NewApp()
	if err != nil {
		log.Fatal(err)
	}
	err = wails.Run(applicationOptions(application, runtime.GOOS))
	if err != nil {
		log.Fatal(err)
	}
}

func applicationOptions(application *App, platform string) *options.App {
	return &options.App{
		Title:     "SSHKing",
		Width:     1240,
		Height:    790,
		MinWidth:  920,
		MinHeight: 600,
		// Windows keeps the custom application chrome while macOS uses Cocoa's
		// native window frame. The latter is what provides the system traffic
		// lights, rounded corners, shadow and full-screen behaviour.
		Frameless:        platform == "windows",
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer:      &assetserver.Options{Assets: assets},
		OnStartup:        application.startup,
		OnShutdown:       application.shutdown,
		Bind:             []interface{}{application},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.Acrylic,
			WebviewGpuIsDisabled: true,
			// Retain DWM's native shadow and Windows 11 rounded corners around
			// the custom title bar.
			DisableFramelessWindowDecorations: false,
			Theme:                             windows.SystemDefault,
		},
		Mac: &mac.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			TitleBar:             mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "SSHKing",
				Message: "A cross-platform SSH workspace.",
			},
			// The frontend uses dark navy text on a light glass surface. Keep the
			// native visual-effect backdrop in the matching Aqua appearance;
			// forcing DarkAqua makes the same text nearly invisible on macOS.
			Appearance: mac.NSAppearanceNameAqua,
		},
	}
}
