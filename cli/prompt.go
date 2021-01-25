package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/jvatic/audible-downloader/audible"
	"github.com/jvatic/audible-downloader/internal/common"
	"github.com/jvatic/audible-downloader/internal/prompt"
	log "github.com/sirupsen/logrus"
)

func PromptChoice(msg string, choices []string) int {
	return prompt.Choice(msg, choices)
}

func PromptString(msg string) string {
	return prompt.String(msg)
}

func PromptPassword(msg string) string {
	return prompt.Password(msg)
}

func PromptCaptcha(imgURL string) string {
	return prompt.String(fmt.Sprintf("%s\nPlease enter captcha from above URL", imgURL))
}

func PromptYesNo(msg string) bool {
	for {
		input := prompt.String(fmt.Sprintf("%s (yes/no)", msg))
		if input == "yes" {
			return true
		}
		if input == "no" {
			return false
		}
		fmt.Println("Please enter yes or no")
	}
}

func RegionPrompt() *audible.Region {
	choices := make([]string, len(audible.Regions))
	for i, r := range audible.Regions {
		choices[i] = r.Name
	}
	ri := PromptChoice("Pick region", choices)
	if ri < 0 {
		ri = 0
	}
	return audible.Regions[ri]
}

func PromptDownloadDir() string {
	dir := PromptString(`Please enter the directory you would like to download to (enter "." for the current directory)`)

	if dir == "." {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			log.Errorf("Error getting current directory: %v", err)
			return PromptDownloadDir()
		}
	}

	dir = filepath.Clean(dir)

	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() {
		log.Error("You've entered an invalid path")
		return PromptDownloadDir()
	}

	return dir
}

func PromptPathTemplate() func(b *audible.Book) string {
	fmt.Printf(`Path template options:
%%TITLE%% - Full book title as seen in library
%%SHORT_TITLE%% - Book title up to the first occurance of a colon (:)
%%AUTHOR%% - Book author(s) (you'll be prompted next for how many to allow and what to separate them with)

default template: %s`, common.DefaultPathTemplate)
	fmt.Println("")
	pathTemplate := PromptString("Please enter a download path template (leave blank for default ^)")
	if pathTemplate == "" {
		pathTemplate = common.DefaultPathTemplate
	}

	var maxAuthors int
	for {
		var err error
		maxAuthors, err = strconv.Atoi(PromptString("Please enter how many authors to allow in %AUTHOR% (0 means unlimited)"))
		if err != nil {
			fmt.Println("Please enter a valid number")
			continue
		}
		break
	}

	var authorSep string
	if maxAuthors == 0 || maxAuthors > 1 {
		authorSep = PromptString(`Please enter what to separate each author with (default: ", ")`)
	}
	if authorSep == "" {
		authorSep = ", "
	}

	getDstPath := common.CompilePathTemplate(
		pathTemplate,
		common.PathTemplateTitle(),
		common.PathTemplateShortTitle(),
		common.PathTemplateAuthor(maxAuthors, authorSep),
	)

	preview := getDstPath(&common.SampleBook)

	fmt.Printf("Preview: %s\n", preview)

	if accept := PromptYesNo("Accept and continue?"); !accept {
		return PromptPathTemplate()
	}

	return getDstPath
}
