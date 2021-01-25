package main

import (
	"image/color"
	"path/filepath"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/container"
)

// FormatFilePath tries to fit the given path within the given width
func FormatFilePath(p string, width int) string {
	textSize := fyne.CurrentApp().Settings().Theme().TextSize()
	textStyle := fyne.TextStyle{Bold: true}
	textWidth := fyne.MeasureText(p, textSize, textStyle).Width
	if textWidth <= width {
		// text will fit without modification
		return p
	}

	parts := strings.SplitAfter(p, string(filepath.Separator))

	ellipsis := "..."
	ellipsisWidth := fyne.MeasureText(ellipsis, textSize, textStyle).Width
	delta := textWidth - width

	for i := 1; i < len(parts)-1; i++ {
		if fyne.MeasureText(parts[i], textSize, textStyle).Width-ellipsisWidth >= delta {
			parts[i] = ellipsis
			return filepath.Join(parts...)
		}
	}

	a := parts[:2]
	var b []string
	c := parts[len(parts)-1:]
	parts = parts[2 : len(parts)-1]
	for i := 0; i < len(parts); i++ {
		if textWidth <= width {
			b = append(b, parts[i:]...)
			break
		}
		textWidth -= fyne.MeasureText(parts[i], textSize, textStyle).Width
	}
	if textWidth > width {
		textWidth -= fyne.MeasureText(a[0], textSize, textStyle).Width
		a = []string{string(filepath.Separator)}
	}
	b = append(b, c...)
	return filepath.Join(filepath.Join(a...), ellipsis, filepath.Join(b...))
}

func Indent(items ...fyne.CanvasObject) fyne.CanvasObject {
	indentStr := ""
	for i := 0; i < 4; i++ {
		indentStr += " "
	}
	return container.NewHBox(
		canvas.NewText(indentStr, color.Black),
		container.NewVBox(items...),
	)
}
