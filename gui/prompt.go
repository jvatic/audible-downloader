package main

import (
	"image"
	"net/http"
	"strings"
	"sync"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/dialog"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
	"github.com/jvatic/audible-downloader/gui/components"
	log "github.com/sirupsen/logrus"
)

func PromptCaptcha(renderQueue chan<- func(w fyne.Window), imgURL string) string {
	resp, err := http.Get(imgURL)
	if err != nil {
		log.Errorf("Error fetching captcha image (%s): %s", imgURL, err)
		return ""
	}
	defer resp.Body.Close()
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		log.Errorf("Error decoding captcha image (%s): %s", imgURL, err)
		return ""
	}

	var d dialog.Dialog
	answer := make(chan string)
	var captcha string
	var captchaMtx sync.RWMutex
	doSubmit := func() {
		captchaMtx.RLock()
		defer captchaMtx.RUnlock()
		answer <- strings.TrimSpace(captcha)
	}

	captchaInput, captchaInCh, captchaOutCh := components.NewEntry(renderQueue,
		components.EntryOptionOnEnter(func() { d.Hide() }),
	)
	defer close(captchaInCh)
	go func() {
		for {
			text, ok := <-captchaOutCh
			if !ok {
				return
			}
			captchaMtx.Lock()
			captcha = text
			captchaMtx.Unlock()
		}
	}()

	captchaImage := canvas.NewImageFromImage(img)
	captchaImage.FillMode = canvas.ImageFillOriginal

	renderQueue <- func(w fyne.Window) {
		d = dialog.NewCustom(
			"Captcha", "Submit",
			fyne.NewContainerWithLayout(
				layout.NewVBoxLayout(),
				components.NewImmutableText("Please enter the letters and numbers in the image below to continue", components.TextOptionBold()),
				captchaImage,
				captchaInput,
			),
			w,
		)
		d.SetOnClosed(doSubmit)
		d.Show()
	}

	return <-answer
}

func PromptString(renderQueue chan<- func(w fyne.Window), msg string) string {
	answer := make(chan string)

	var d dialog.Dialog
	var inputText string
	var inputTextMtx sync.RWMutex
	doSubmit := func() {
		inputTextMtx.RLock()
		defer inputTextMtx.RUnlock()
		answer <- strings.TrimSpace(inputText)
	}

	textInput, textInCh, textOutCh := components.NewEntry(renderQueue,
		components.EntryOptionOnEnter(func() { d.Hide() }),
	)
	defer close(textInCh)

	go func() {
		for {
			text, ok := <-textOutCh
			if !ok {
				return
			}
			inputTextMtx.Lock()
			inputText = text
			inputTextMtx.Unlock()
		}
	}()

	renderQueue <- func(w fyne.Window) {
		d = dialog.NewCustom(
			"", "Submit",
			fyne.NewContainerWithLayout(
				layout.NewVBoxLayout(),
				components.NewImmutableText(msg, components.TextOptionBold()),
				textInput,
			),
			w,
		)
		d.SetOnClosed(doSubmit)
		d.Show()
	}

	return <-answer
}

func PromptChoice(renderQueue chan<- func(w fyne.Window), msg string, options []string) int {
	answer := make(chan int)

	var idx int
	doSubmit := func() {
		answer <- idx
	}

	renderQueue <- func(w fyne.Window) {
		selectInput := widget.NewSelect(options, func(s string) {
			for i, o := range options {
				if o == s {
					idx = i
					return
				}
			}
		})

		d := dialog.NewCustom(
			"", "Submit",
			fyne.NewContainerWithLayout(
				layout.NewVBoxLayout(),
				components.NewImmutableText(msg, components.TextOptionBold()),
				selectInput,
			),
			w,
		)
		d.SetOnClosed(doSubmit)
		d.Show()
	}

	return <-answer
}
