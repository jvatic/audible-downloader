package audible

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/jvatic/audible-downloader/internal/cookiejar"
	log "github.com/sirupsen/logrus"
)

type Option func(*Client)

func OptionBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.rawBaseURL = baseURL
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

func OptionPromptChoice(getChoice func(msg string, opts []string) int) Option {
	return func(c *Client) {
		c.getChoice = getChoice
	}
}

func OptionPlayerID(playerID string) Option {
	return func(c *Client) {
		c.playerID = playerID
	}
}

type Client struct {
	*http.Client
	jar            http.CookieJar
	rawBaseURL     string
	baseURL        *url.URL
	baseLicenseURL string
	username       string
	password       string
	playerID       string
	getAuthCode    func() string
	getCaptcha     func(imgURL string) string
	getChoice      func(msg string, opts []string) int
	lastURL        *url.URL
}

func NewClient(opts ...Option) (*Client, error) {
	c := &Client{
		playerID:       generatePlayerID(),
		baseLicenseURL: "https://www.audible.com",
	}

	// persistent cookiejar that wraps net/http/cookiejar.Jar
	jar, err := cookiejar.NewJar()
	if err != nil {
		return nil, err
	}
	c.jar = jar

	c.Client = &http.Client{
		Jar: c.jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > 10 {
				return http.ErrUseLastResponse
			}
			log.TraceFn(func() []interface{} {
				return []interface{}{fmt.Sprintf("Redirect: %s", req.URL)}
			})
			c.lastURL = req.URL
			return nil
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	u, err := url.Parse(c.rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("Valid BaseURL is required")
	}
	c.baseURL = u
	c.Client.Transport = &roundTripper{}

	if c.username == "" {
		log.Warn("Username is empty")
	}

	if c.password == "" {
		log.Warn("Password is empty")
	}

	return c, nil
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if !req.URL.IsAbs() {
		// Allow client to make requests relative to baseURL
		req.URL.Scheme = c.baseURL.Scheme
		req.URL.Host = c.baseURL.Host
	}

	return c.Client.Do(req)
}
