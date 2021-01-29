package main

import (
	"context"
	"sort"

	"fyne.io/fyne"
	"github.com/jvatic/audible-downloader/audible"
	"github.com/jvatic/audible-downloader/gui/library"
	"github.com/jvatic/audible-downloader/gui/signin"
	log "github.com/sirupsen/logrus"
)

type RenderFunc func(w fyne.Window)

type Controller struct {
	render chan func(w fyne.Window)
	done   chan struct{}
}

func NewController() *Controller {
	return &Controller{
		render: make(chan func(w fyne.Window)),
		done:   make(chan struct{}),
	}
}

func (c *Controller) Run(w fyne.Window) {
	go func() {
		// render loop
		for {
			select {
			case fn, ok := <-c.render:
				if !ok {
					return
				}
				fn(w)
				break
			case <-c.done:
				break
			}
		}
	}()

	// main logic

	// {
	// 	// TODO: remove this block
	// 	books, err := LoadLibrary()
	// 	if err == nil {
	// 		sort.Sort(audible.ByTitle(books))
	// 		stateCh := NewLibState(&audible.Client{}, []byte{}, books)
	// 		if err := Library(w, c.render, stateCh); err != nil {
	// 			ShowFatalErrorDialog(c.render, err)
	// 			return
	// 		}
	// 	} else {
	// 		log.Warn(err)
	// 	}
	// }

	client, err := signin.Run(c.render)
	if err != nil {
		ShowFatalErrorDialog(c.render, err)
		return
	}

	ctx := context.Background()
	activationBytes, err := client.GetActivationBytes(ctx)
	if err != nil {
		log.Errorf("Error getting activation bytes: %s", err)
		return
	}
	log.Debugf("Activation Bytes: %s\n", string(activationBytes))

	books, err := client.GetLibrary(ctx)
	if err != nil {
		log.Errorf("Error reading library: %s\n", err)
		return
	}
	sort.Sort(audible.ByTitle(books))
	{
		// TODO: remove this block
		if err := library.SaveLibrary(books); err != nil {
			log.Warn(err)
		}
	}

	stateCh := library.NewState(client, activationBytes, books)
	if err := library.Run(w, c.render, stateCh); err != nil {
		ShowFatalErrorDialog(c.render, err)
		return
	}

	// nothing more to do so close window and quit
	w.Close()
}

func (c *Controller) Stop() {
	close(c.done)
}
