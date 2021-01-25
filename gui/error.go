package main

import (
	"image/color"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/dialog"
	log "github.com/sirupsen/logrus"
)

func ShowFatalErrorDialog(w fyne.Window, err error) {
	log.Error(err)
	d := dialog.NewCustom(
		"Error", "Quit",
		canvas.NewText(err.Error(), color.Black),
		w,
	)
	d.SetOnClosed(func() {
		w.Close()
	})
	d.Show()
}
