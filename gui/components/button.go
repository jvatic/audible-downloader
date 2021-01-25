package components

import (
	"fyne.io/fyne"
	"fyne.io/fyne/widget"
)

type ButtonOption func(b *widget.Button)

func ButtonOptionIcon(icon fyne.Resource) ButtonOption {
	return func(b *widget.Button) {
		b.Icon = icon
	}
}

func ButtonOptionImportance(importance widget.ButtonImportance) ButtonOption {
	return func(b *widget.Button) {
		b.Importance = importance
	}
}

func ButtonOptionAlignment(alignment widget.ButtonAlign) ButtonOption {
	return func(b *widget.Button) {
		b.Alignment = alignment
	}
}

func ButtonOptionIconPlacement(placement widget.ButtonIconPlacement) ButtonOption {
	return func(b *widget.Button) {
		b.IconPlacement = placement
	}
}

func ButtonOptionOnTapped(fn func()) ButtonOption {
	return func(b *widget.Button) {
		b.OnTapped = fn
	}
}

type ButtonAction func(b *widget.Button)

func ButtonActionSetText(text string) ButtonAction {
	return func(b *widget.Button) {
		b.SetText(text)
	}
}

func ButtonActionDisable() ButtonAction {
	return func(b *widget.Button) {
		b.Disable()
	}
}

func ButtonActionEnable() ButtonAction {
	return func(b *widget.Button) {
		b.Enable()
	}
}

func ButtonActionShow() ButtonAction {
	return func(b *widget.Button) {
		b.Show()
	}
}

func ButtonActionHide() ButtonAction {
	return func(b *widget.Button) {
		b.Hide()
	}
}

func NewButton(label string, opts ...ButtonOption) (*widget.Button, chan<- ButtonAction) {
	button := widget.NewButton(label, nil)
	for _, fn := range opts {
		fn(button)
	}

	ch := make(chan ButtonAction)
	go func() {
		for {
			fn, ok := <-ch
			if !ok {
				return
			}
			fn(button)
		}
	}()

	return button, ch
}
