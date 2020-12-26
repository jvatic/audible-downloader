package audible

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	log "github.com/sirupsen/logrus"
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

func OptionCookieJar(jar http.CookieJar) Option {
	return func(c *Client) {
		c.jar = jar
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
	baseURL        string
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

	// setup http client with cookie jar
	if c.jar == nil {
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			return nil, err
		}
		c.jar = jar
	}

	c.Client = &http.Client{
		Jar: c.jar,
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
	c.Client.Transport = &roundTripper{baseURL: u, jar: c.jar}

	if c.username == "" {
		log.Warn("Username is empty")
	}

	if c.password == "" {
		log.Warn("Password is empty")
	}

	return c, nil
}
