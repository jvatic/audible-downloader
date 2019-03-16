package auth

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
	"golang.org/x/net/publicsuffix"
)

type Option func(*Client)

func OptionBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

func OptionUsername(username string) Option {
	return func(c *Client) {
		c.username = username
	}
}

func OptionPassword(password string) Option {
	return func(c *Client) {
		c.password = password
	}
}

func OptionAuthCode(getAuthCode func() string) Option {
	return func(c *Client) {
		c.getAuthCode = getAuthCode
	}
}

func OptionCaptcha(getCaptcha func(imgURL string) string) Option {
	return func(c *Client) {
		c.getCaptcha = getCaptcha
	}
}

func OptionLang(lang string) Option {
	return func(c *Client) {
		c.lang = lang
	}
}

func OptionPlayerID(playerID string) Option {
	return func(c *Client) {
		c.playerID = playerID
	}
}

type Client struct {
	*http.Client
	lang           string
	baseURL        string
	baseLicenseURL string
	username       string
	password       string
	playerID       string
	getAuthCode    func() string
	getCaptcha     func(imgURL string) string
	lastURL        *url.URL
}

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

func generatePlayerID() string {
	h := sha1.New()
	h.Write([]byte("P1"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		lang:           "us",
		playerID:       generatePlayerID(),
		baseLicenseURL: "https://www.audible.com",
	}

	// setup http client with cookie jar
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}

	c.Client = &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 10 {
				return http.ErrUseLastResponse
			}
			c.lastURL = req.URL
			return nil
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("Valid BaseURL is required")
	}
	c.Client.Transport = &roundTripper{baseURL: u, jar: jar}

	if c.username == "" {
		return nil, fmt.Errorf("Username is required")
	}

	if c.password == "" {
		return nil, fmt.Errorf("Password is required")
	}

	return c, nil
}

func parseForm(form *html.Node) (string, url.Values) {
	actionURL := htmlquery.SelectAttr(form, "action")
	data := url.Values{}
	for _, input := range htmlquery.Find(form, "//input") {
		data.Set(htmlquery.SelectAttr(input, "name"), htmlquery.SelectAttr(input, "value"))
	}
	return actionURL, data
}

func (c *Client) submitForm(pageURL string, actionURL string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequest("POST", actionURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", pageURL)
	return c.Do(req)
}

func (c *Client) Authenticate() error {
	// fetch landing page
	resp, err := c.Get("https://www.audible.ca/en_CA/?ipRedirectOverride=true")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return err
	}

	// handle IP redirects
	if a := htmlquery.FindOne(doc, "//a[contains(@href, 'RedirectOverride=true')]"); a != nil {
		resp, err = c.Get(htmlquery.SelectAttr(a, "href"))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		doc, err = htmlquery.Parse(resp.Body)
		if err != nil {
			return err
		}
	}

	// find sign in URL
	var signinURL string
	if a := htmlquery.FindOne(doc, "//a[contains(@class, 'ui-it-sign-in-link')]"); a != nil {
		signinURL = htmlquery.SelectAttr(a, "href")
	} else {
		return fmt.Errorf("Unable to find sign in link")
	}

	// fetch sign in page
	resp, err = c.Get(signinURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	doc, err = htmlquery.Parse(resp.Body)
	if err != nil {
		return err
	}

	// Sign in
	var actionURL string
	var data url.Values
	if form := htmlquery.FindOne(doc, "//form[@name = 'signIn']"); form != nil {
		actionURL, data = parseForm(form)
	} else {
		return fmt.Errorf("Unable to parse form action")
	}
	// add username and password to form values
	data.Set("email", c.username)
	data.Set("password", c.password)
	// submit form
	resp, err = c.submitForm(resp.Request.URL.String(), actionURL, data)
	if err != nil {
		return fmt.Errorf("error posting username and password: %s", err)
	}
	defer resp.Body.Close()
	doc, err = htmlquery.Parse(resp.Body)
	if err != nil {
		return err
	}

	// Handle captcha
	for {
		if img := htmlquery.FindOne(doc, "//img[@id = 'auth-captcha-image']"); img != nil {
			if div := htmlquery.FindOne(doc, "//div[contains(@id, '-message-box')]"); div != nil {
				for _, line := range strings.Split(strings.TrimSpace(htmlquery.InnerText(div)), "\n") {
					line = strings.TrimSpace(line)
					if line != "" {
						fmt.Println(line)
					}
				}
			}

			var captcha string
			if c.getCaptcha != nil {
				captcha = c.getCaptcha(htmlquery.SelectAttr(img, "src"))
			} else {
				return fmt.Errorf("captcha encountered and OptionCaptcha not given")
			}

			if form := htmlquery.FindOne(doc, "//form[@name = 'signIn']"); form != nil {
				actionURL, data = parseForm(form)
			} else {
				return fmt.Errorf("Unable to parse form action")
			}
			// add username and password to form values
			data.Set("email", c.username)
			data.Set("password", c.password)
			// add captcha
			data.Set("guess", captcha)
			// submit form
			resp, err = c.submitForm(resp.Request.URL.String(), actionURL, data)
			if err != nil {
				return fmt.Errorf("error posting username and password: %s", err)
			}
			defer resp.Body.Close()
			doc, err = htmlquery.Parse(resp.Body)
			if err != nil {
				return err
			}
		} else {
			break
		}
	}

	// Submit OTP code if promted for one
	if form := htmlquery.FindOne(doc, "//form[@id = 'auth-mfa-form']"); form != nil {
		actionURL, data := parseForm(form)
		if input := htmlquery.FindOne(doc, "//input[@name = 'otpCode']"); input != nil {
			// OTP enabled
			if c.getAuthCode == nil {
				return fmt.Errorf("OTP enabled on account and OptionAuthCode not given")
			}
			data.Set("otpCode", c.getAuthCode())
			resp, err = c.submitForm(resp.Request.URL.String(), actionURL, data)
			if err != nil {
				return fmt.Errorf("error posting auth code: %s", err)
			}
			defer resp.Body.Close()
			doc, err = htmlquery.Parse(resp.Body)
			if err != nil {
				return err
			}
		}
	}

	// We're authenticated now!
	return nil
}

func (c *Client) GetActivationBytes() ([]byte, error) {
	query := url.Values{
		"ipRedirectOverride": []string{"true"},
		"playerType":         []string{"software"},
		"bp_ua":              []string{"y"},
		"playerModel":        []string{"Desktop"},
		"playerId":           []string{c.playerID},
		"playerManufacturer": []string{"Audible"},
		"serial":             []string{""},
	}
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	u.RawQuery = query.Encode()
	u.Path = "/player-auth-token"
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", admUserAgent)
	_, err = c.Do(req)
	if err != nil {
		return nil, err
	}

	playerToken := c.lastURL.Query().Get("playerToken")
	if playerToken == "" {
		return nil, fmt.Errorf("Unable to get player token")
	}

	deregister := func() error {
		u, err := url.Parse(c.baseLicenseURL)
		if err != nil {
			return err
		}
		query := url.Values{
			"customer_token": []string{playerToken},
			"action":         []string{"de-register"},
		}
		u.RawQuery = query.Encode()
		u.Path = "/license/licenseForCustomerToken"
		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", admUserAgent)
		_, err = c.Do(req)
		return err
	}

	if err := deregister(); err != nil {
		return nil, err
	}

	u, err = url.Parse(c.baseLicenseURL)
	if err != nil {
		return nil, err
	}
	query = url.Values{
		"customer_token": []string{playerToken},
	}
	u.RawQuery = query.Encode()
	u.Path = "/license/licenseForCustomerToken"

	req, err = http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", admUserAgent)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := deregister(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)

	file, err := os.OpenFile("activation.blob", os.O_RDWR|os.O_CREATE, 0755)
	if err == nil {
		io.Copy(file, bytes.NewReader(buf.Bytes()))
		file.Close()
	}

	return ExtractActivationBytes(&buf)
}

// adapted from https://github.com/mkb79/Audible/blob/a1a7041b2f67f11e6e55dd86dc2eee00f01c5593/src/audible/activation_bytes.py#L41-L68
func ExtractActivationBytes(data io.Reader) ([]byte, error) {
	version := ""
	scanner := bufio.NewScanner(data)
	var prev []byte
	for scanner.Scan() {
		line := scanner.Bytes()

		// extract metadata (such lines are in the form "(key=value)" and come
		// before the binary data)
		if strings.HasPrefix(string(line), "(") {
			kv := strings.SplitN(strings.TrimSuffix(strings.TrimPrefix(string(line), "("), ")"), "=", 2)
			if len(kv) == 2 && kv[0] == "version" {
				// we only need to check the version
				version = kv[1]
			}
			continue
		}

		// make sure (version=1), our extraction process may need updating for any
		// future versions
		if version != "1" {
			return nil, fmt.Errorf("Expected version=1, got version=%s", version)
		}

		// pick up first part of the current key (see next block)
		if prev != nil {
			line = append(prev, line...)
			prev = nil
		}

		// handle case where a key has a newline
		if len(line) < 70 {
			prev = line
			prev = append(prev, []byte("\n")...)
			continue
		}

		// make sure the key is 70 bytes
		if len(line) != 70 {
			break
		}

		h := make([]byte, hex.EncodedLen(len(line)))
		hex.Encode(h, line)

		// only 8 bytes of the first key are necessary for decryption
		// get the endianness right (reverse in pairs of 2)
		activationBytes := make([]byte, 0, 8)
		for i := 6; i >= 0; i -= 2 {
			activationBytes = append(activationBytes, h[i:i+2]...)
		}
		return activationBytes, nil
	}
	return nil, fmt.Errorf("Invalid input data")
}
