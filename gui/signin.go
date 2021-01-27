package main

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"fyne.io/fyne"
	"fyne.io/fyne/layout"
	"fyne.io/fyne/widget"
	"github.com/jvatic/audible-downloader/audible"
	"github.com/jvatic/audible-downloader/gui/components"
	log "github.com/sirupsen/logrus"
)

func SignIn(w fyne.Window, renderQueue chan<- func(w fyne.Window)) (*audible.Client, error) {
	var loading bool
	var username, password string
	var usernameMtx, passwordMtx sync.RWMutex
	var region *audible.Region
	submitChan := make(chan struct{})

	var build func() fyne.CanvasObject
	render := func() func(w fyne.Window) {
		return func(w fyne.Window) {
			w.SetContent(
				components.ApplyTemplate(build()),
			)
		}
	}
	build = func() fyne.CanvasObject {
		submitEnabled := false
		doSubmit := func() {
			if !submitEnabled {
				return
			}
			usernameMtx.RLock()
			defer usernameMtx.RUnlock()
			passwordMtx.RLock()
			defer passwordMtx.RUnlock()
			if username == "" || password == "" || region == nil {
				return
			}
			loading = true
			renderQueue <- render()
			close(submitChan)
		}

		usernameInput, usernameInCh, usernameOutCh := components.NewEntry(
			components.EntryOptionOnEnter(doSubmit),
			components.EntryOptionPlaceholder("yourusername@example.com"),
		)
		usernameInCh <- username

		passwordInput, passwordInCh, passwordOutCh := components.NewEntry(
			components.EntryOptionOnEnter(doSubmit),
			components.EntryOptionPassword(),
			components.EntryOptionPlaceholder("Password"),
		)
		passwordInCh <- password

		go func() {
			for {
				select {
				case v, ok := <-usernameOutCh:
					if !ok {
						return
					}
					usernameMtx.Lock()
					username = v
					usernameMtx.Unlock()
					break
				case v, ok := <-passwordOutCh:
					if !ok {
						return
					}
					passwordMtx.Lock()
					password = v
					passwordMtx.Unlock()
					break
				}
			}
		}()

		cancelBtn := widget.NewButton("Quit", func() {
			log.Debug("Signin canceled")
			w.Close()
		})

		submitBtn := widget.NewButton("Sign in", doSubmit)
		submitBtn.Importance = widget.HighImportance
		submitBtn.Disable()
		if loading {
			submitBtn.SetText("Signing in...")
		}

		handleFormChanged := func() {
			if username != "" && password != "" && region != nil {
				submitEnabled = true
				submitBtn.Enable()
			} else {
				submitBtn.Disable()
			}
		}
		usernameInput.OnChanged = func(s string) {
			username = s
			handleFormChanged()
		}
		passwordInput.OnChanged = func(s string) {
			password = s
			handleFormChanged()
		}

		regionOptions := make([]string, len(audible.Regions))
		for i, r := range audible.Regions {
			regionOptions[i] = r.Name
		}
		regionSelect := widget.NewSelect(regionOptions, func(s string) {
			for _, r := range audible.Regions {
				if r.Name == s {
					region = r
					handleFormChanged()
					return
				}
			}
		})
		if region != nil {
			regionSelect.Selected = region.Name
		}

		if loading {
			usernameInput.Disable()
			passwordInput.Disable()
			regionSelect.Disable()
			cancelBtn.Disable()
		}

		return fyne.NewContainerWithLayout(
			layout.NewHBoxLayout(),
			layout.NewSpacer(),
			fyne.NewContainerWithLayout(
				layout.NewVBoxLayout(),
				layout.NewSpacer(),
				fyne.NewContainerWithLayout(
					layout.NewCenterLayout(),
					components.NewImmutableText("Sign in with your Amazon account", components.TextOptionHeading(components.H1)),
				),
				fyne.NewContainerWithLayout(
					layout.NewVBoxLayout(),
					components.NewImmutableText("E-mail address or mobile phone number", components.TextOptionBold()),
					usernameInput,
					components.NewImmutableText("Password", components.TextOptionBold()),
					passwordInput,
					components.NewImmutableText("Region", components.TextOptionBold()),
					regionSelect,
				),
				fyne.NewContainerWithLayout(
					layout.NewHBoxLayout(),
					layout.NewSpacer(),
					cancelBtn,
					submitBtn,
				),
				layout.NewSpacer(),
			),
			layout.NewSpacer(),
		)
	}
	renderQueue <- render()

	<-submitChan
	return doSignin(w, username, password, region)
}

func doSignin(w fyne.Window, username, password string, region *audible.Region) (*audible.Client, error) {
	u, err := url.Parse(fmt.Sprintf("https://www.audible.%s", region.TLD))
	if err != nil {
		return nil, fmt.Errorf("Unable to parse domain: %w", err)
	}
	log.Infof("Using %s (%s)\n", u.Host, region.Name)

	client, err := audible.NewClient(
		audible.OptionBaseURL(u.String()),
		audible.OptionUsername(username),
		audible.OptionPassword(password),
		audible.OptionCaptcha(func(imgURL string) string {
			return PromptCaptcha(w, imgURL)
		}),
		audible.OptionAuthCode(func() string {
			return PromptString(w, "Auth Code (OTP)")
		}),
		audible.OptionPromptChoice(func(msg string, opts []string) int {
			return PromptChoice(w, msg, opts)
		}),
	)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	if err := client.Authenticate(ctx); err != nil {
		return nil, fmt.Errorf("Error authenticating: %w\n", err)
	}

	return client, nil
}
