package components

import (
	"fyne.io/fyne/widget"
)

type CheckboxOption = func(cb *widget.Check)

func CheckboxOptionOnChange(fn func(checked bool)) CheckboxOption {
	return func(cb *widget.Check) {
		cb.OnChanged = fn
	}
}

type CheckboxAction = func(cb *widget.Check)

func CheckboxActionSetChecked(checked bool) CheckboxAction {
	return func(cb *widget.Check) {
		cb.Checked = checked
		cb.Refresh()
	}
}

func CheckboxActionEnable() CheckboxAction {
	return func(cb *widget.Check) {
		cb.Enable()
	}
}

func CheckboxActionDisable() CheckboxAction {
	return func(cb *widget.Check) {
		cb.Disable()
	}
}

func IsCheckboxDisabled(ch chan<- CheckboxAction) bool {
	valCh := make(chan bool)
	ch <- func(cb *widget.Check) {
		valCh <- cb.Disabled()
	}
	disabled := <-valCh
	close(valCh)
	return disabled
}

func IsCheckboxChecked(ch chan<- CheckboxAction) bool {
	valCh := make(chan bool)
	ch <- func(cb *widget.Check) {
		valCh <- cb.Checked
	}
	checked := <-valCh
	close(valCh)
	return checked
}

func NewCheckbox(label string, opts ...CheckboxOption) (*widget.Check, chan<- CheckboxAction) {
	cb := widget.NewCheck(label, nil)
	for _, fn := range opts {
		fn(cb)
	}

	ch := make(chan CheckboxAction)
	go func() {
		for {
			fn, ok := <-ch
			if !ok {
				return
			}

			fn(cb)
		}
	}()

	return cb, ch
}
