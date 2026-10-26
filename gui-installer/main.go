package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title: "ServeHub-2 Installer",
		Width: 1440,
		Height: 960,
		MinWidth: 960,
		MinHeight: 640,
		WindowStartState: options.Maximised,
		Frameless: true,
		Icon: appIcon,
		BackgroundColour: options.NewRGB(22, 24, 27),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
