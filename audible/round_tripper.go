package audible

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

type roundTripper struct {
	baseURL *url.URL
	jar     http.CookieJar
}

const admUserAgent = "Audible Download Manager"

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if !req.URL.IsAbs() {
		// Allow client to make requests relative to baseURL
		req.URL.Scheme = rt.baseURL.Scheme
		req.URL.Host = rt.baseURL.Host
		req.Header.Set("Host", req.URL.Host)
	}
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

	t := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       4 * time.Minute,
		TLSHandshakeTimeout:   2 * time.Minute,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// send captured cookies with request
	for _, c := range rt.jar.Cookies(req.URL) {
		log.Tracef("AddCookie(%s): %v", req.URL, c)
		req.AddCookie(c)
	}

	log.Debugf("%s %s", req.Method, req.URL)
	log.TraceFn(logHeader(req.Header, "User-Agent"))

	resp, err := t.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	log.Debugf("-> %s: %s", req.URL, resp.Status)
	log.TraceFn(logHeader(resp.Header, "Content-Type"))

	// capture response cookies
	rt.jar.SetCookies(resp.Request.URL, resp.Cookies())
	if strings.Contains(resp.Request.URL.Host, "amazon.") {
		// make sure amazon.com has cookies
		if u, err := url.Parse("https://amazon.com"); err == nil {
			cookies := resp.Cookies()
			log.Tracef("SetCookies(%s): %v", u, cookies)
			rt.jar.SetCookies(u, cookies)
		}
	}

	log.TraceFn(logResponseBody(resp))

	return resp, nil
}

type readCloser struct {
	io.Reader
}

func (b *readCloser) Close() error {
	return nil
}

func logResponseBody(resp *http.Response) log.LogFunction {
	return func() []interface{} {
		var buf bytes.Buffer
		io.Copy(&buf, resp.Body)
		body := buf.String()
		// allow reading the body again
		resp.Body = &readCloser{Reader: &buf}
		return []interface{}{body}
	}
}

func logHeader(h http.Header, name string) log.LogFunction {
	return func() []interface{} {
		return []interface{}{fmt.Sprintf("%s: %s", name, h.Get(name))}
	}
}
