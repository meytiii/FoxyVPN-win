package main

import (
	"embed"
	"fmt"
	"os"

	"foxyvpn/pkg/logger"
	"foxyvpn/pkg/winutil"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Single instance mutex lock
	lock, err := winutil.AcquireSingleInstance()
	if err != nil {
		fmt.Println("Another instance of FoxyVPN is already running.")
		logger.GetLogger().Warn("Main", "Another instance of FoxyVPN is already running. Exiting.")
		os.Exit(0)
	}
	defer lock.Release()

	app := NewApp()

	err = wails.Run(&options.App{
		Title:         "FoxyVPN",
		Width:         430,
		Height:        720,
		MinWidth:      380,
		MinHeight:     640,
		DisableResize: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 11, G: 13, B: 20, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		EnableDefaultContextMenu: true,
		Bind: []interface{}{
			app,
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: false,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			BackdropType:         windows.Mica,
			Theme:                windows.Dark,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
