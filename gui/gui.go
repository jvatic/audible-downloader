package main

import (
	"context"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/dialog"
	"github.com/jvatic/audible-downloader/internal/common"
	"github.com/jvatic/audible-downloader/internal/config"
	log "github.com/sirupsen/logrus"
)

func main() {
	if err := config.Init(); err != nil {
		log.Fatalf("Error: %s", err)
	}

	ctx := common.InitShutdownSignals(context.Background())

	app.New()
	mainWindow := fyne.CurrentApp().NewWindow("Audible Downloader")
	mainWindow.Resize(fyne.Size{Width: 1000, Height: 500})
	controller := NewController()
	go controller.Run(mainWindow)
	defer controller.Stop()

	mainWindow.SetCloseIntercept(func() {
		dialog.ShowConfirm(
			"Are you sure you want to exit?",
			"You may resume your downloads later.",
			func(response bool) {
				if response {
					mainWindow.Close()
				}
			}, mainWindow)
	})

	showAndRun(ctx, mainWindow)
}

func showAndRun(ctx context.Context, w fyne.Window) {
	app := fyne.CurrentApp()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			app.Quit()
		case <-done:
		}
	}()

	w.ShowAndRun()
}
