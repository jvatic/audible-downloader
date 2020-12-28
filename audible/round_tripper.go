package audible

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/jvatic/audible-downloader/internal/config"
	log "github.com/sirupsen/logrus"
)

type roundTripper struct {
}

const admUserAgent = "Audible Download Manager"

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") != admUserAgent {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:83.0) Gecko/20100101 Firefox/83.0")
	}
	if req.Header.Get("Accept") == "" && req.Header.Get("User-Agent") != admUserAgent {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	}
	if req.Header.Get("Accept-Language") == "" {
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	}

	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("DNT", "1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	log.Debugf("%s %s", req.Method, req.URL)
	log.TraceFn(logHeader(req.Header, "User-Agent"))

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	log.Debugf("-> %s: %s", resp.Request.URL, resp.Status)
	log.TraceFn(logHeader(resp.Header, "Content-Type"))
	log.TraceFn(logResponseBody(resp))

	return resp, nil
}

type readCloser struct {
	io.Reader
}

func (b *readCloser) Close() error {
	return nil
}

var nBody int
var nBodyMtx sync.Mutex

func logResponseBody(resp *http.Response) log.LogFunction {
	return func() []interface{} {
		nBodyMtx.Lock()
		defer nBodyMtx.Unlock()
		nBody++
		var buf bytes.Buffer
		io.Copy(&buf, resp.Body)
		// allow reading the body again
		resp.Body = &readCloser{Reader: &buf}
		t, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil {
			t = "text/plain"
		}
		exts, err := mime.ExtensionsByType(t)
		if err != nil {
			exts = []string{".txt"}
		}
		p := filepath.Join(config.Dir(), "debug", fmt.Sprintf("%02d%s", nBody, exts[0]))
		os.MkdirAll(filepath.Dir(p), 0755)
		file, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			return []interface{}{fmt.Sprintf("error saving response body to %q: %s", p, err)}
		}
		io.Copy(file, bytes.NewReader(buf.Bytes()))
		return []interface{}{fmt.Sprintf("response body saved to %q", p)}
	}
}

func logHeader(h http.Header, name string) log.LogFunction {
	return func() []interface{} {
		return []interface{}{fmt.Sprintf("%s: %s", name, h.Get(name))}
	}
}
