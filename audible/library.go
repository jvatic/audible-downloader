package audible

import (
	"context"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/antchfx/htmlquery"
	"github.com/jvatic/audible-downloader/internal/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
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
	ThumbImage   image.Image `json:"-"`
	AudibleURL   string
	LocalPath    string
}

func (b *Book) ID() string {
	parts := strings.Split(b.AudibleURL, "/")
	if len(parts) == 0 {
		return b.AudibleURL
	}
	return parts[len(parts)-1]
}

func (b *Book) WriteInfo(w io.Writer) error {
	_, err := fmt.Fprintln(w, b.Title)
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(w, "Written by: ")
	if err != nil {
		return err
	}
	for i, name := range b.Authors {
		if i > 0 {
			_, err = fmt.Fprint(w, ", ")
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprint(w, name)
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(w, "")
	if err != nil {
		return err
	}

	_, err = fmt.Fprint(w, "Narrated by: ")
	if err != nil {
		return err
	}
	for i, name := range b.Narrators {
		if i > 0 {
			_, err = fmt.Fprint(w, ", ")
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprint(w, name)
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(w, "")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "URL: %s", b.AudibleURL)
	return err
}

// ByTitle implements sort.Interface for []*Book based on the Title field
type ByTitle []*Book

func (a ByTitle) Len() int           { return len(a) }
func (a ByTitle) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByTitle) Less(i, j int) bool { return strings.Compare(a[i].Title, a[j].Title) < 0 }

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

		nextPageURL, err := url.Parse(page.NextPageURL)
		if err != nil {
			break outer
		}

		pageNumber := nextPageURL.Query().Get("page")
		if pageNumber == "" {
			break outer
		}

		for _, n := range visited {
			if pageNumber == n {
				break outer
			}
		}

		visited = append(visited, pageNumber)

		page, err = c.getLibraryPage(ctx, page.NextPageURL)
		if err != nil {
			return nil, err
		}
	}

	// make sure we have exactly one entry per book
	bookURLs := make(map[string]struct{}, len(books))
	dedupedBooks := make([]*Book, 0, len(books))
	for _, b := range books {
		if _, ok := bookURLs[b.AudibleURL]; ok {
			continue
		}
		bookURLs[b.AudibleURL] = struct{}{}
		dedupedBooks = append(dedupedBooks, b)
	}
	books = dedupedBooks

	var wg sync.WaitGroup
	for _, b := range books {
		wg.Add(1)
		go func(b *Book) {
			defer wg.Done()
			resp, err := c.Get(b.ThumbURL)
			if err != nil {
				log.Errorf("error fetching ThumbURL(%s): %s", b.ThumbURL, err)
				return
			}
			defer resp.Body.Close()
			img, _, err := image.Decode(resp.Body)
			if err != nil {
				log.Errorf("error decoding ThumbURL(%s): %s", b.ThumbURL, err)
				return
			}
			b.ThumbImage = img
		}(b)
	}
	wg.Wait()

	return books, nil
}

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

	anchorElements := htmlquery.Find(doc, "//a]")
	for _, a := range anchorElements {
		if strings.TrimSpace(htmlquery.InnerText(a)) == "Go forward a page" {
			page.NextPageURL = htmlquery.SelectAttr(a, "href")
			break
		}
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
					book.AudibleURL = strings.Split(u.String(), "?")[0]
				}
			}
			book.Title = strings.TrimSpace(htmlquery.InnerText(node))
		}

		foundAuthors := false
		foundNarrators := false
		for _, li := range htmlquery.Find(row, "//li") {
			if foundAuthors && foundNarrators {
				break
			}
			text := strings.TrimSpace(htmlquery.InnerText(li))
			if strings.HasPrefix(text, "Written by:") {
				authors := []string{}
				for _, a := range htmlquery.Find(li, "//a") {
					authors = append(authors, strings.TrimSpace(htmlquery.InnerText(a)))
				}
				book.Authors = authors
				foundAuthors = true
				continue
			}
			if strings.HasPrefix(text, "Narrated by:") {
				narrators := []string{}
				for _, a := range htmlquery.Find(li, "//a") {
					narrators = append(narrators, strings.TrimSpace(htmlquery.InnerText(a)))
				}
				book.Narrators = narrators
				foundNarrators = true
				continue
			}
		}

		book.DownloadURLs = make(map[string]string)
		addDownloadURL := func(a *html.Node) error {
			href, err := url.Parse(htmlquery.SelectAttr(a, "href"))
			if err != nil {
				return err
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
			return nil
		}

		for _, a := range htmlquery.Find(row, "//a[contains(@href, '/download?')]") {
			addDownloadURL(a)
		}
		if a := htmlquery.FindOne(row, "//a[contains(@href, '/companion-file/')]"); a != nil {
			addDownloadURL(a)
		}

		if book.AudibleURL == "" && len(book.DownloadURLs) > 0 {
			for _, du := range book.DownloadURLs {
				log.Debugf("using download URL (%s) for AudibleURL", du)
				if u, err := url.Parse(du); err == nil {
					q := u.Query()
					if asin := q.Get("asin"); asin != "" {
						book.AudibleURL = fmt.Sprintf("https://audible.com/pd/%s", asin)
					} else {
						log.Debugf("unable to find asin in url(%s)", du)
					}
				} else {
					log.Debugf("error parsing url(%s): %v", du, err)
				}
				break
			}
		}

		page.Books = append(page.Books, book)
	}

	return page, nil
}
