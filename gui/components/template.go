package components

import (
	"fyne.io/fyne"
	"fyne.io/fyne/layout"
)

func ApplyTemplate(o fyne.CanvasObject) fyne.CanvasObject {
	return fyne.NewContainerWithLayout(
		layout.NewMaxLayout(),
		o,
	)
}
