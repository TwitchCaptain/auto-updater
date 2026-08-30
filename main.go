// Package main is Captain Updater, a Windows tray app that upgrades GitHub-released executables.
package main

import (
	"embed"
	"log"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"

	"github.com/TwitchCaptain/auto-updater/internal/config"
	"github.com/TwitchCaptain/auto-updater/internal/history"
	"github.com/TwitchCaptain/auto-updater/internal/paths"
	"github.com/TwitchCaptain/auto-updater/internal/service"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appicon []byte

func init() {
	application.RegisterEvent[string]("open-app")
	application.RegisterEvent[string]("history-updated")
	application.RegisterEvent[[]string]("files-dropped")
}

func main() {
	cfgPath, err := paths.ConfigFile()
	if err != nil {
		log.Fatal(err)
	}

	histPath, err := paths.HistoryFile()
	if err != nil {
		log.Fatal(err)
	}

	store := config.New(cfgPath)
	if err := store.Detect(); err != nil {
		log.Fatal(err)
	}

	svc := service.New(store, history.New(histPath))

	icon := appicon
	if len(icon) == 0 {
		icon = icons.SystrayMacTemplate
	}

	app := application.New(application.Options{
		Name:        "Captain Updater",
		Description: "Keep Windows apps current from GitHub releases",
		Services: []application.Service{
			application.NewService(svc),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.twitchcaptain.updater",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				for _, arg := range data.Args {
					if strings.Contains(strings.ToLower(arg), "captainupdater://") {
						svc.HandleProtocol(arg)

						return
					}
				}

				svc.ShowWindow()
			},
		},
		OnShutdown: func() {
			svc.Stop()
		},
	})

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Captain Updater",
		Width:            1080,
		Height:           720,
		MinWidth:         720,
		MinHeight:        480,
		Hidden:           true,
		EnableFileDrop:   true,
		BackgroundColour: application.NewRGB(24, 26, 27),
		URL:              "/",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar: false,
		},
	})

	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		win.Hide()
		e.Cancel()
	})

	win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		app.Event.Emit("files-dropped", files)
	})

	tray := app.SystemTray.New()
	tray.SetLabel("Captain Updater")
	tray.SetIcon(icon)
	tray.AttachWindow(win).WindowOffset(8)

	menu := app.NewMenu()
	menu.Add("Open").OnClick(func(_ *application.Context) {
		svc.ShowWindow()
	})
	menu.Add("Check now").OnClick(func(_ *application.Context) {
		go svc.CheckAll()
	})
	menu.Add("Lock").OnClick(func(_ *application.Context) {
		svc.Lock()
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(_ *application.Context) {
		svc.Stop()
		app.Quit()
	})
	tray.SetMenu(menu)

	svc.SetApp(app, win)

	go watchActivate(svc)

	for _, arg := range os.Args[1:] {
		if strings.Contains(strings.ToLower(arg), "captainupdater://") {
			svc.HandleProtocol(arg)
			win.Show()
		}
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func watchActivate(svc *service.Service) {
	path, err := paths.ActivateFile()
	if err != nil {
		return
	}

	t := time.NewTicker(800 * time.Millisecond)
	defer t.Stop()

	for range t.C {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		_ = os.Remove(path)
		raw := strings.TrimSpace(string(b))
		if raw != "" {
			svc.HandleProtocol(raw)
		} else {
			svc.ShowWindow()
		}
	}
}
