package common

import (
	"bytes"
	"context"
	"image"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dhowden/tag"
	"github.com/jvatic/audible-downloader/audible"
	"github.com/jvatic/audible-downloader/internal/utils"
	log "github.com/sirupsen/logrus"
)

func InitShutdownSignals(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	// SIGINT or SIGTERM cancels ctx, triggering a graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigs:
			cancel()
			signal.Reset() // repeated signals will have default behaviour
		case <-ctx.Done():
		}
	}()
	return ctx
}

var DefaultPathTemplate = filepath.Join("%AUTHOR%", "%SHORT_TITLE%", "%TITLE%.mp4")

var SampleBook = audible.Book{
	Title:   "A Basic Book Title: And subtitle",
	Authors: []string{"First Author", "Second Author", "Third Author"},
}

type PathTemplateSub func(p string, b *audible.Book) string

func PathTemplateTitle() PathTemplateSub {
	tmpl := "%TITLE%"
	return func(p string, b *audible.Book) string {
		return strings.ReplaceAll(p, tmpl, b.Title)
	}
}

func PathTemplateShortTitle() PathTemplateSub {
	tmpl := "%SHORT_TITLE%"
	return func(p string, b *audible.Book) string {
		return strings.ReplaceAll(p, tmpl, strings.SplitN(b.Title, ":", 2)[0])
	}
}

func PathTemplateAuthor(max int, sep string) PathTemplateSub {
	tmpl := "%AUTHOR%"
	return func(p string, b *audible.Book) string {
		var n int
		if max == 0 {
			n = len(b.Authors)
		} else {
			n = utils.MinInt(max, len(b.Authors))
		}
		authors := b.Authors[0:n]
		return strings.ReplaceAll(p, tmpl, strings.Join(authors, sep))
	}
}

// CompilePathTemplate combines given template and PathTemplateSub fns into a
// fn taking an *audible.Book and returning the destination path
func CompilePathTemplate(t string, subs ...PathTemplateSub) func(b *audible.Book) string {
	return func(b *audible.Book) string {
		p := t
		for _, fn := range subs {
			p = fn(p, b)
		}
		return p
	}
}

func ListDownloadedBooks(dir string) ([]*audible.Book, error) {
	parseAuthors := func(str string) []string {
		parts := strings.Split(str, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		return parts
	}
	booksByID := make(map[string]*audible.Book)
	books := make([]*audible.Book, 0)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		isMP4 := filepath.Ext(path) == ".mp4"
		file, err := os.Open(path)
		if err != nil {
			if isMP4 {
				log.Warnf("Unable to read %s: %s", path, err)
			}
			return nil
		}
		defer file.Close()
		if _, _, err := tag.Identify(file); err != nil {
			if isMP4 {
				log.Warnf("Unable to identify %s: %s", path, err)
			}
			return nil
		}
		meta, err := tag.ReadAtoms(file)
		if err != nil {
			if isMP4 {
				log.Warnf("Unable to read tag data %s: %s", path, err)
			}
			return nil
		}
		if strings.ToLower(meta.Genre()) != "audiobook" {
			return nil
		}
		var thumbImg image.Image
		if p := meta.Picture(); p != nil {
			thumbImg, _, _ = image.Decode(bytes.NewReader(p.Data))
		}
		b := &audible.Book{
			Title:      meta.Title(),
			Authors:    parseAuthors(meta.Artist()),
			ThumbImage: thumbImg,
			LocalPath:  path,
		}
		if eb := booksByID[b.ID()]; eb != nil {
			// don't overwrite an mp4 entry (e.g. with an aax one)
			if filepath.Ext(eb.LocalPath) == ".mp4" {
				return nil
			}
		}
		books = append(books, b)
		booksByID[b.ID()] = b
		return nil
	})
	if err != nil {
		return nil, err
	}
	return books, nil
}

func WriteInfoFile(dir string, book *audible.Book) error {
	f, err := os.OpenFile(filepath.Join(dir, "info.txt"), os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	return book.WriteInfo(f)
}
