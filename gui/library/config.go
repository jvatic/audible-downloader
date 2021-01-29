package library

import (
	"image/color"
	"sync"

	"fyne.io/fyne"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/container"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
	"github.com/jvatic/audible-downloader/gui/components"
	"github.com/jvatic/audible-downloader/internal/common"
)

func buildConfigUI(renderQueue chan<- func(w fyne.Window), actionQueue chan<- Action, closeFunc func()) fyne.CanvasObject {
	var pathTemplateMtx sync.RWMutex
	pathTemplate := common.DefaultPathTemplate
	setPathTemplate := func(text string) {
		pathTemplateMtx.Lock()
		defer pathTemplateMtx.Unlock()
		pathTemplate = text
	}

	getPathTemplate := func() string {
		pathTemplateMtx.RLock()
		defer pathTemplateMtx.RUnlock()
		return pathTemplate
	}

	var maxAuthorsMtx sync.RWMutex
	maxAuthors := 1
	setMaxAuthors := func(num int) {
		maxAuthorsMtx.Lock()
		defer maxAuthorsMtx.Unlock()
		maxAuthors = num
	}

	getMaxAuthors := func() int {
		maxAuthorsMtx.RLock()
		defer maxAuthorsMtx.RUnlock()
		return maxAuthors
	}

	var authorSeparatorMtx sync.RWMutex
	authorSeparator := ", "
	setAuthorSeparator := func(sep string) {
		authorSeparatorMtx.Lock()
		defer authorSeparatorMtx.Unlock()
		authorSeparator = sep
	}

	getAuthorSeparator := func() string {
		authorSeparatorMtx.RLock()
		defer authorSeparatorMtx.RUnlock()
		return authorSeparator
	}

	updatePathTemplateSub := func() {
		actionQueue <- func(s *State) {
			authorSeparatorMtx.RLock()
			defer authorSeparatorMtx.RUnlock()
			maxAuthorsMtx.RLock()
			defer maxAuthorsMtx.RUnlock()
			s.getDstPath = common.CompilePathTemplate(
				getPathTemplate(),
				common.PathTemplateTitle(),
				common.PathTemplateShortTitle(),
				common.PathTemplateAuthor(maxAuthors, authorSeparator),
			)
		}
	}

	previewText, previewTextCh := components.NewText(renderQueue, "")
	updatePreviewText := func() {
		previewTextCh <- GetDstPath(actionQueue, &common.SampleBook)
	}
	updatePreviewText()

	pathTemplateInput, pathTemplateInputInCh, pathTemplateInputOutCh := components.NewEntry(renderQueue)
	pathTemplateInputInCh <- getPathTemplate()
	go func() {
		for {
			text, ok := <-pathTemplateInputOutCh
			if !ok {
				return
			}
			setPathTemplate(text)
			updatePathTemplateSub()
			updatePreviewText()
		}
	}()

	maxAuthorsInput, maxAuthorsInputInCh, maxAuthorsInputOutCh := components.NewIntEntry(renderQueue)
	maxAuthorsInputInCh <- getMaxAuthors()
	go func() {
		for {
			num, ok := <-maxAuthorsInputOutCh
			if !ok {
				return
			}
			setMaxAuthors(num)
			updatePathTemplateSub()
			updatePreviewText()
		}
	}()

	authorSeparatorInput, authorSeparatorInputInCh, authorSeparatorInputOutCh := components.NewEntry(renderQueue)
	authorSeparatorInputInCh <- getAuthorSeparator()
	go func() {
		for {
			sep, ok := <-authorSeparatorInputOutCh
			if !ok {
				return
			}
			setAuthorSeparator(sep)
			updatePathTemplateSub()
			updatePreviewText()
		}
	}()

	return components.ApplyTemplate(
		container.NewVBox(
			container.NewCenter(
				components.NewImmutableText("Download Settings", components.TextOptionHeading(components.H1)),
			),
			container.NewVBox(
				components.NewImmutableText("Download Path Template", components.TextOptionBold()),
				pathTemplateInput,
				Indent(
					components.NewImmutableText("Format Options: ", components.TextOptionBold()),
					container.NewHBox(
						components.NewImmutableText("%TITLE%", components.TextOptionBold()),
						canvas.NewText(" - Full book title as seen in library", color.Black),
					),
					container.NewHBox(
						components.NewImmutableText("%SHORT_TITLE%", components.TextOptionBold()),
						canvas.NewText(" - Book title up to the first occurance of ", color.Black),
						components.NewImmutableText(":", components.TextOptionBold()),
					),
					components.NewImmutableText("%AUTHOR%", components.TextOptionBold()),
					Indent(
						container.NewHBox(
							components.NewImmutableText("Max number of authors to include (0 = unlimited): ", components.TextOptionBold()),
							maxAuthorsInput,
							components.NewImmutableText("Author separator: ", components.TextOptionBold()),
							authorSeparatorInput,
						),
					),
				),
				components.NewImmutableText("Preview: ", components.TextOptionBold()),
				previewText,
			),
			layout.NewSpacer(),
			container.NewHBox(
				layout.NewSpacer(),
				widget.NewButton("Cancel", closeFunc),
				widget.NewButton("Save", closeFunc),
			),
		),
	)
}
