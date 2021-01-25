package audible

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"

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

func OptionPromptChoice(getChoice func(msg string, opts []string) int) Option {
	return func(c *Client) {
		c.getChoice = getChoice
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
	getChoice      func(msg string, opts []string) int
	lastURL        *url.URL
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
