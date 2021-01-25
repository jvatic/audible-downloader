package components

import (
	"image/color"
	"strings"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/container"
)

type TextOption func(t *canvas.Text)

func TextOptionColor(c color.Color) TextOption {
	return func(t *canvas.Text) {
		t.Color = c
	}
}

func TextOptionSize(size int) TextOption {
	return func(t *canvas.Text) {
		t.TextSize = size
	}
}

type HeadingLevel int

const (
	H1 HeadingLevel = iota
	H2
	H3
	H4
	H5
	H6
)

func TextOptionHeading(level HeadingLevel) TextOption {
	var size int
	switch level {
	case H1:
		size = 32
		break
	case H2:
		size = 24
		break
	case H3:
		size = 18
		break
	case H4:
		size = 16
		break
	case H5:
		size = 14
		break
	case H6:
		size = 12
		break
	}
	return func(t *canvas.Text) {
		TextOptionBold()(t)
		TextOptionSize(size)(t)
	}
}

func TextOptionBold() TextOption {
	return func(t *canvas.Text) {
		t.TextStyle = fyne.TextStyle{Bold: true}
	}
}

func TextOptionStyle(style fyne.TextStyle) TextOption {
	return func(t *canvas.Text) {
		t.TextStyle = style
	}
}

func TextOptionAlignment(alignment fyne.TextAlign) TextOption {
	return func(t *canvas.Text) {
		t.Alignment = alignment
	}
}

func NewText(text string, opts ...TextOption) (*canvas.Text, chan<- string) {
	t := canvas.NewText(text, fyne.CurrentApp().Settings().Theme().TextColor())
	for _, fn := range opts {
		fn(t)
	}

	ch := make(chan string)
	go func() {
		for {
			text, ok := <-ch
			if !ok {
				return
			}

			t.SetText(text)
			t.Refresh()
		}
	}()

	return t, ch
}

func NewImmutableText(text string, opts ...TextOption) *canvas.Text {
	t := canvas.NewText(text, fyne.CurrentApp().Settings().Theme().TextColor())
	for _, fn := range opts {
		fn(t)
	}
	return t
}

func NewWrappedText(text string, width int, opts ...TextOption) (fyne.CanvasObject, chan<- string) {
	container := container.NewVBox()
	ch := make(chan string)

	wto := &canvas.Text{
		TextSize: fyne.CurrentApp().Settings().Theme().TextSize(),
		Color:    fyne.CurrentApp().Settings().Theme().TextColor(),
	}
	for _, fn := range opts {
		fn(wto)
	}

	textSize := wto.TextSize
	textStyle := wto.TextStyle
	color := wto.Color
	alignment := wto.Alignment

	var lines []*canvas.Text
	renderText := func(text string) {
		// split text into rows
		chunks := strings.Split(text, " ")
		lineNum := -1
		for i := len(chunks); i > 0; i-- {
			rowText := strings.Join(chunks[0:i], " ")
			if i > 0 && fyne.MeasureText(rowText, textSize, textStyle).Width > width {
				continue
			}
			lineNum++

			if lineNum < len(lines) {
				line := lines[lineNum]
				line.Text = rowText
			} else {
				line := canvas.NewText(rowText, color)
				line.TextSize = textSize
				line.Alignment = alignment
				line.TextStyle = textStyle
				lines = append(lines, line)
				container.Add(line)
			}

			chunks = chunks[i:]
			i = len(chunks) + 1
		}

		for i, line := range lines {
			if i > lineNum {
				container.Remove(line)
			} else {
				line.Refresh()
			}
		}
		lines = lines[0 : lineNum+1]
	}

	go func() {
		for {
			text, ok := <-ch
			if !ok {
				return
			}

			renderText(text)
		}
	}()

	renderText(text)

	return container, ch
}
