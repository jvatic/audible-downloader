package audible

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/jvatic/audible-downloader/internal/utils"
)

type Page struct {
	Books       []*Book
	NextPageURL string
}

type Book struct {
	Title        string
	Authors      []string
	Narrators    []string
	Duration     time.Duration
	DownloadURLs map[string]string
	ThumbURL     string
	AudibleURL   string
}

func (b *Book) Dir() string {
	dirName := ""
	for i, name := range b.Authors {
		if i > 0 {
			dirName += ", "
		}
		dirName += utils.NormalizeFilename(name)
	}
	if dirName == "" {
		dirName = "Unknown Author"
	}
	return filepath.Join(dirName, utils.NormalizeFilename(b.Title))
}

func (b *Book) WriteInfo(w io.Writer) error {
	_, err := fmt.Fprintln(w, b.Title)

	_, err = fmt.Fprint(w, "Written by: ")
	for i, name := range b.Authors {
		if i > 0 {
			_, err = fmt.Fprint(w, ", ")
		}
		_, err = fmt.Fprint(w, name)
	}
	_, err = fmt.Fprintln(w, "")

	_, err = fmt.Fprint(w, "Narrated by: ")
	for i, name := range b.Narrators {
		if i > 0 {
			_, err = fmt.Fprint(w, ", ")
		}
		_, err = fmt.Fprint(w, name)
	}
	_, err = fmt.Fprintln(w, "")

	_, err = fmt.Fprintf(w, "URL: %s", b.AudibleURL)
	return err
}

func (c *Client) GetLibrary(ctx context.Context) ([]*Book, error) {
	page, err := c.getLibraryPage(ctx, "/lib")
	if err != nil {
		return nil, err
	}
	books := make([]*Book, 0, len(page.Books))
	var visited []string
outer:
	for {
		books = append(books, page.Books...)

		if page.NextPageURL == "" {
			break outer
		}

		for _, u := range visited {
			if page.NextPageURL == u {
				break outer
			}
		}

		visited = append(visited, page.NextPageURL)
		page, err = c.getLibraryPage(ctx, page.NextPageURL)
		if err != nil {
			return nil, err
		}
	}

	return books, nil
}

var nSaved int = 0

func (c *Client) getLibraryPage(ctx context.Context, pageURL string) (*Page, error) {
	// fetch library page
	reqCtx := utils.ContextWithCancelChan(context.Background(), ctx.Done())
	req, err := http.NewRequestWithContext(reqCtx, "GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	page := &Page{}

	paginationAnchors := htmlquery.Find(doc, "//a[@data-name = 'page']")
	if len(paginationAnchors) > 0 {
		page.NextPageURL = htmlquery.SelectAttr(paginationAnchors[len(paginationAnchors)-1], "href")
	}

	for _, row := range htmlquery.Find(doc, "//div[contains(@id, 'adbl-library-content-row-')]") {
		book := &Book{}

		if img := htmlquery.FindOne(row, "//img[contains(@src, '.jpg')]"); img != nil {
			book.ThumbURL = htmlquery.SelectAttr(img, "src")
		}

		// the title is always in the first <li>
		if node := htmlquery.FindOne(row, "//li"); node != nil {
			if a := htmlquery.FindOne(node, "/a"); a != nil {
				// save a link to the book on Audible
				if u, err := url.Parse(htmlquery.SelectAttr(a, "href")); err == nil {
					u = resp.Request.URL.ResolveReference(u)
					u.RawQuery = "" // query is unnecessary baggage
					book.AudibleURL = u.String()
				}
			}
			book.Title = strings.TrimSpace(htmlquery.InnerText(node))
		}

		if node := htmlquery.FindOne(row, "//li[contains(@class, 'authorLabel')]"); node != nil {
			authors := []string{}
			for _, a := range htmlquery.Find(node, "//a") {
				authors = append(authors, strings.TrimSpace(htmlquery.InnerText(a)))
			}
			book.Authors = authors
		}

		if node := htmlquery.FindOne(row, "//li[contains(@class, 'narratorLabel')]"); node != nil {
			narrators := []string{}
			for _, a := range htmlquery.Find(node, "//a") {
				narrators = append(narrators, strings.TrimSpace(htmlquery.InnerText(a)))
			}
			book.Narrators = narrators
		}

		book.DownloadURLs = make(map[string]string)
		if col := htmlquery.FindOne(row, "//div[contains(@class, 'adbl-library-action')]"); col != nil {
			for _, a := range htmlquery.Find(col, "//a") {
				href, err := url.Parse(htmlquery.SelectAttr(a, "href"))
				if err != nil {
					continue
				}
				if !href.IsAbs() {
					href = resp.Request.URL.ResolveReference(href)
				}
				text := strings.TrimSpace(htmlquery.InnerText(a))
				hasURL := false
				for _, du := range book.DownloadURLs {
					if du == href.String() {
						hasURL = true
						break
					}
				}
				if !hasURL {
					book.DownloadURLs[text] = href.String()
				}
			}
		}

		page.Books = append(page.Books, book)
	}

	return page, nil
}