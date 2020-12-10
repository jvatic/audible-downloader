package audible

import (
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type roundTripper struct {
	baseURL *url.URL
	jar     *cookiejar.Jar
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
		req.AddCookie(c)
	}

	resp, err := t.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// capture response cookies
	rt.jar.SetCookies(resp.Request.URL, resp.Cookies())
	if strings.Contains(resp.Request.URL.Host, "amazon.") {
		// make sure amazon.com has cookies
		if u, err := url.Parse("https://amazon.com"); err == nil {
			rt.jar.SetCookies(u, resp.Cookies())
		}
	}

	return resp, nil
}
