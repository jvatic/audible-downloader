package audible

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/antchfx/htmlquery"
	"github.com/antchfx/xpath"
	"github.com/jvatic/audible-downloader/internal/config"
	log "github.com/sirupsen/logrus"
)

var redactEnabled = true

func init() {
	if v := os.Getenv("REDACT_DISABLE"); v == "true" {
		redactEnabled = false
	}
}

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

	log.Debugf("%s %s", req.Method, redactURL(req.URL))
	log.TraceFn(logHeader(req.Header, "User-Agent"))

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	log.Debugf("-> %s: %s", redactURL(resp.Request.URL), resp.Status)
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
		if err != nil || len(exts) < 1 {
			exts = []string{".txt"}
		}
		p := filepath.Join(config.Dir(), "debug", fmt.Sprintf("%02d%s", nBody, exts[0]))
		os.MkdirAll(filepath.Dir(p), 0755)
		file, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			return []interface{}{fmt.Sprintf("error saving response body to %q: %s", p, err)}
		}
		if strings.HasPrefix(exts[0], ".htm") {
			io.Copy(file, bytes.NewReader(redactHTML(buf.Bytes())))
		} else {
			io.Copy(file, bytes.NewReader(buf.Bytes()))
		}
		return []interface{}{fmt.Sprintf("response body saved to %q", p)}
	}
}

func logHeader(h http.Header, name string) log.LogFunction {
	return func() []interface{} {
		return []interface{}{fmt.Sprintf("%s: %s", name, h.Get(name))}
	}
}

func redactURL(u *url.URL) *url.URL {
	if !redactEnabled {
		return u
	}
	redacted := &url.URL{
		Scheme:   u.Scheme,
		Opaque:   u.Opaque,
		Host:     u.Host,
		Path:     u.Path,
		RawQuery: redactQuery(u.Query()).Encode(),
	}
	return redacted
}

var redactQueryAllowlist = map[string]bool{"ipRedirectOverride": true}

func redactQuery(q url.Values) url.Values {
	if !redactEnabled {
		return q
	}
	redacted := make(url.Values, len(q))
	for k, v := range q {
		if allowed, ok := redactQueryAllowlist[k]; allowed && ok {
			redacted[k] = v
		} else {
			redacted[k] = []string{"REDACTED"}
		}
	}
	return redacted
}

var (
	findScriptTags   = xpath.MustCompile("//script")
	findHiddenInputs = xpath.MustCompile(`//input[@type="hidden"]`)
)

func redactHTML(data []byte) []byte {
	if !redactEnabled {
		return data
	}
	doc, err := htmlquery.Parse(bytes.NewReader(data))
	if err != nil {
		return data
	}

	// remove all <script /> elements
	for _, node := range htmlquery.QuerySelectorAll(doc, findScriptTags) {
		node.Parent.RemoveChild(node)
	}

	// remove all hidden <input /> elements
	for _, node := range htmlquery.QuerySelectorAll(doc, findHiddenInputs) {
		node.Parent.RemoveChild(node)
	}

	return []byte(htmlquery.OutputHTML(doc, true))
}
