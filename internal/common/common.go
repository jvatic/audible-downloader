package common

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
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

type bookInfo struct {
	Title     string
	Authors   []string
	Narrators []string
	URL       string
}

func parseInfoTxt(data io.Reader) (*bookInfo, error) {
	info := &bookInfo{}

	s := bufio.NewScanner(data)

	// the first line is the title
	s.Scan()
	info.Title = s.Text()

	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "Written by:") {
			info.Authors = strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "Written by:")), ", ")
			continue
		}
		if strings.HasPrefix(line, "Narrated by:") {
			info.Narrators = strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "Narrated by:")), ", ")
			continue
		}
		if strings.HasPrefix(line, "URL:") {
			info.URL = strings.TrimSpace(strings.TrimPrefix(line, "URL:"))
			continue
		}
	}

	if len(info.Title) == 0 {
		return nil, fmt.Errorf("invalid info.txt: missing book title")
	}

	if len(info.Authors) == 0 {
		return nil, fmt.Errorf("invalid info.txt: missing book authors")
	}

	if len(info.Narrators) == 0 {
		return nil, fmt.Errorf("invalid info.txt: missing book narrators")
	}

	if len(info.URL) == 0 {
		return nil, fmt.Errorf("invalid info.txt: missing book URL")
	}

	if err := s.Err(); err != nil {
		return nil, err
	}

	return info, nil
}

func ListDownloadedBooks(dir string) ([]*audible.Book, error) {
	booksByID := make(map[string]*audible.Book)
	books := make([]*audible.Book, 0)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// identify existing books by their info.txt files
		if filepath.Base(path) != "info.txt" {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			log.Warnf("Unable to read %s: %s", path, err)
			return err
		}
		defer file.Close()
		bookInfo, err := parseInfoTxt(file)
		if err != nil {
			log.Warnf("Unable to parse book info (%s): %s", path, err)
			return err
		}

		b := &audible.Book{
			Title:      bookInfo.Title,
			Authors:    bookInfo.Authors,
			Narrators:  bookInfo.Narrators,
			AudibleURL: bookInfo.URL,
		}

		isMP4 := false
		exts := []string{".mp4", ".aax"}
		m, err := filepath.Glob(filepath.Join(filepath.Dir(path), "*"))
		if err != nil {
			log.Debugf("unable to glob %s: %v", path, err)
			return err
		}
		for _, e := range m {
			for _, ext := range exts {
				if strings.Contains(e, ext) {
					if filepath.Ext(e) == ".mp4" {
						isMP4 = true
					}
					e = strings.TrimSuffix(e, ".icloud")
					b.LocalPath = e
					log.Debugf("setting local path for book(%s): %s", b.ID(), e)
				}
			}
		}

		if b.LocalPath == "" {
			log.Warnf("unable to find audio file for %s", path)
			return nil
		}

		if eb := booksByID[b.ID()]; eb != nil {
			// don't overwrite an mp4 entry (e.g. with an aax one)
			if filepath.Ext(eb.LocalPath) == ".mp4" {
				return nil
			}
		}
		books = append(books, b)
		booksByID[b.ID()] = b

		if !isMP4 {
			return nil
		}

		mp4File, err := os.Open(b.LocalPath)
		if err != nil {
			log.Warnf("Unable to read %s: %s", path, err)
			return nil
		}
		defer mp4File.Close()
		if _, _, err := tag.Identify(mp4File); err != nil {
			log.Warnf("Unable to identify %s: %s", path, err)
			return nil
		}

		meta, err := tag.ReadAtoms(mp4File)
		if err != nil {
			log.Warnf("Unable to read tag data %s: %s", path, err)
			return nil
		}
		if strings.ToLower(meta.Genre()) != "audiobook" {
			return nil
		}

		if title := meta.Title(); title != "" {
			b.Title = title
		}

		var thumbImg image.Image
		if p := meta.Picture(); p != nil {
			thumbImg, _, _ = image.Decode(bytes.NewReader(p.Data))
		}
		b.ThumbImage = thumbImg

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
