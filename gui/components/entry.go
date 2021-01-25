package components

import (
	"fmt"
	"strconv"

	"fyne.io/fyne"
	"fyne.io/fyne/widget"
)

type entry struct {
	widget.Entry
	typedKey  func(key *fyne.KeyEvent)
	typedRune func(r rune)
}

func (e *entry) TypedKey(key *fyne.KeyEvent) {
	if e.typedKey == nil {
		e.Entry.TypedKey(key)
	} else {
		// typedKey is expected to call e.Entry.TypedKey when not overriding
		e.typedKey(key)
	}
}

func (e *entry) TypedRune(r rune) {
	if e.typedRune == nil {
		e.Entry.TypedRune(r)
	} else {
		// typedRune is expected to call e.Entry.TypedRune when not overriding
		e.typedRune(r)
	}
}

type EntryOption func(e *entry)

func EntryOptionMultiline() EntryOption {
	return func(e *entry) {
		e.MultiLine = true
	}
}

func EntryOptionPassword() EntryOption {
	return func(e *entry) {
		e.Password = true
	}
}

func EntryOptionPlaceholder(text string) EntryOption {
	return func(e *entry) {
		e.PlaceHolder = text
	}
}

func EntryOptionOnEnter(fn func()) EntryOption {
	return func(e *entry) {
		nextFn := e.typedKey
		if nextFn == nil {
			// default behaviour
			nextFn = e.Entry.TypedKey
		}
		e.typedKey = func(key *fyne.KeyEvent) {
			switch key.Name {
			case fyne.KeyReturn:
				if fn != nil {
					fn()
				}
			default:
				nextFn(key)
			}
		}
	}
}

// EntryOptionInt forces input to be a valid int
func EntryOptionInt() EntryOption {
	return func(e *entry) {
		nextFn := e.typedRune
		if nextFn == nil {
			// default behaviour
			nextFn = e.Entry.TypedRune
		}
		e.typedRune = func(r rune) {
			_, err := strconv.Atoi(string(r))
			if err != nil {
				return
			}
			nextFn(r)
		}
	}
}

func NewEntry(opts ...EntryOption) (*entry, chan<- string, <-chan string) {
	entry := &entry{}
	entry.ExtendBaseWidget(entry)
	for _, fn := range opts {
		fn(entry)
	}

	inCh := make(chan string)
	outCh := make(chan string)

	go func() {
		for {
			text, ok := <-inCh
			if !ok {
				close(outCh)
				return
			}

			entry.SetText(text)
		}
	}()

	entry.OnChanged = func(text string) {
		outCh <- text
	}

	return entry, inCh, outCh
}

func NewIntEntry(opts ...EntryOption) (*entry, chan<- int, <-chan int) {
	opts = append([]EntryOption{EntryOptionInt()}, opts...)
	entry, inStrCh, outStrCh := NewEntry(opts...)

	inCh := make(chan int)
	outCh := make(chan int)

	go func() {
		for {
			num, ok := <-inCh
			if !ok {
				close(outCh)
				close(inStrCh)
				return
			}

			inStrCh <- fmt.Sprintf("%d", num)
		}
	}()

	go func() {
		for {
			text, ok := <-outStrCh
			if !ok {
				return
			}

			num, err := strconv.Atoi(text)
			if err == nil {
				outCh <- num
			}
		}
	}()

	return entry, inCh, outCh
}
