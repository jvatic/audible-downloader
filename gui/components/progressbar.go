package components

import "fyne.io/fyne/widget"

type ProgressBarOption = func(pb *widget.ProgressBar)

type ProgressBarAction = func(pb *widget.ProgressBar)

func ProgressBarActionShow() ProgressBarAction {
	return func(pb *widget.ProgressBar) {
		pb.Show()
	}
}

func ProgressBarActionHide() ProgressBarAction {
	return func(pb *widget.ProgressBar) {
		pb.Hide()
	}
}

func ProgressBarActionSetComplete() ProgressBarAction {
	return ProgressBarActionSetValue(1.0)
}

func ProgressBarActionSetValue(val float64) ProgressBarAction {
	return func(pb *widget.ProgressBar) {
		pb.SetValue(val)
	}
}

func IsProgressBarHidden(pbCh chan<- ProgressBarAction) bool {
	ch := make(chan bool)
	pbCh <- func(pb *widget.ProgressBar) {
		ch <- pb.Hidden
	}
	hidden := <-ch
	close(ch)
	return hidden
}

// TODO make it actually work
func NewProgressBar(opts ...ProgressBarOption) (*widget.ProgressBar, chan<- ProgressBarAction) {
	pb := widget.NewProgressBar()
	for _, fn := range opts {
		fn(pb)
	}

	ch := make(chan ProgressBarAction)
	go func() {
		for {
			fn, ok := <-ch
			if !ok {
				return
			}

			fn(pb)
		}
	}()

	return pb, ch
}
