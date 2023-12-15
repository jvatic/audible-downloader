package audible

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/jvatic/audible-downloader/internal/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
)

func (c *Client) Authenticate(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	s := &authState{c: c}

	// we're not authenticated, so run through the full signin process
	steps := []authStep{
		s.getLandingPage,
		s.getSigninPage,
		s.doSignin,
		s.handleCaptcha,
		s.handleOTPSelection,
		s.handleOTP,
		s.handleCaptcha,
		s.confirmAuth,
	}

	for i, step := range steps {
		if err := step(ctx); err != nil {
			return fmt.Errorf("step[%d]: %w", i, err)
		}
	}

	// We're authenticated now!
	return nil
}

func (c *Client) GetPlayerToken(ctx context.Context) (string, error) {
	log.Debug("GetPlayerToken")

	query := url.Values{
		"ipRedirectOverride": []string{"true"},
		"playerType":         []string{"software"},
		"bp_ua":              []string{"y"},
		"playerModel":        []string{"Desktop"},
		"playerId":           []string{c.playerID},
		"playerManufacturer": []string{"Audible"},
		"serial":             []string{""},
	}
	u, err := url.Parse(c.rawBaseURL)
	if err != nil {
		return "", err
	}
	u.RawQuery = query.Encode()
	u.Path = "/player-auth-token"
	reqCtx := utils.ContextWithCancelChan(context.Background(), ctx.Done())
	req, err := http.NewRequestWithContext(reqCtx, "GET", u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", admUserAgent)
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()

	playerToken := c.lastURL.Query().Get("playerToken")
	if playerToken == "" {
		return "", fmt.Errorf("unable to get player token")
	}

	log.Debugf("PlayerToken: %s", playerToken)

	return playerToken, nil
}

func (c *Client) GetActivationBytes(ctx context.Context) ([]byte, error) {
	log.Debug("GetActivationBytes")

	playerToken, err := c.GetPlayerToken(ctx)
	if err != nil {
		return nil, err
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
		reqCtx := utils.ContextWithCancelChan(context.Background(), ctx.Done())
		req, err := http.NewRequestWithContext(reqCtx, "GET", u.String(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", admUserAgent)
		resp, err := c.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		return nil
	}

	if err := deregister(); err != nil {
		return nil, err
	}

	u, err := url.Parse(c.baseLicenseURL)
	if err != nil {
		return nil, err
	}
	query := url.Values{
		"customer_token": []string{playerToken},
	}
	u.RawQuery = query.Encode()
	u.Path = "/license/licenseForCustomerToken"

	reqCtx := utils.ContextWithCancelChan(context.Background(), ctx.Done())
	req, err := http.NewRequestWithContext(reqCtx, "GET", u.String(), nil)
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
			return nil, fmt.Errorf("expected version=1, got version=%s", version)
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
	return nil, fmt.Errorf("invalid input data")
}

func generatePlayerID() string {
	h := sha1.New()
	h.Write([]byte("P1"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

type authStep func(context.Context) error

type authState struct {
	c            *Client
	doc          *html.Node
	lastResponse *http.Response
}

func (s *authState) PageURL() *url.URL {
	if s.lastResponse != nil {
		return s.lastResponse.Request.URL
	}
	return nil
}

func (s *authState) PageURLString() string {
	u := s.PageURL()
	if u != nil {
		return u.String()
	}
	return ""
}

func (s *authState) parseForm(form *html.Node) (string, url.Values) {
	actionURL := strings.TrimSpace(htmlquery.SelectAttr(form, "action"))
	if u, err := url.Parse(actionURL); err == nil {
		if s.lastResponse != nil {
			actionURL = s.lastResponse.Request.URL.ResolveReference(u).String()
		}
	}
	data := url.Values{}
	for _, input := range htmlquery.Find(form, "//input") {
		data.Set(htmlquery.SelectAttr(input, "name"), htmlquery.SelectAttr(input, "value"))
	}
	return actionURL, data
}

func (s *authState) submitForm(ctx context.Context, actionURL string, data url.Values) (*http.Response, error) {
	reqCtx := utils.ContextWithCancelChan(context.Background(), ctx.Done())
	req, err := http.NewRequestWithContext(reqCtx, "POST", actionURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", s.PageURLString())
	return s.c.Do(req)
}

func (s *authState) getMessageBoxString() string {
	var heading string
	var messages []string
	if div := htmlquery.FindOne(s.doc, "//div[contains(@id, '-message-box')]"); div != nil {
		if h := htmlquery.FindOne(div, "//h4"); h != nil {
			heading = strings.TrimSpace(htmlquery.InnerText(h))
		}
		for _, li := range htmlquery.Find(div, "//ul/li") {
			if str := strings.TrimSpace(htmlquery.InnerText(li)); str != "" {
				messages = append(messages, str)
			}
		}
		return fmt.Sprintf("%s: %s", heading, strings.Join(messages, ", "))
	}
	return ""
}

func (s *authState) getLandingPage(ctx context.Context) error {
	log.Debug("getLandingPage / override IP Redirect")

	req, err := http.NewRequestWithContext(
		utils.ContextWithCancelChan(context.Background(), ctx.Done()),
		"GET", "/?ipRedirectOverride=true", nil)
	if err != nil {
		return err
	}
	resp, err := s.c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return err
	}
	s.doc = doc
	s.lastResponse = resp
	return nil
}

func (s *authState) getSigninPage(ctx context.Context) error {
	log.Debug("getSigninPage")

	// find sign in URL
	var signinURL string
	if a := htmlquery.FindOne(s.doc, "//a[contains(@class, 'ui-it-sign-in-link')]"); a != nil {
		signinURL = htmlquery.SelectAttr(a, "href")
	} else {
		return fmt.Errorf("unable to find sign in link")
	}

	// fetch sign in page
	req, err := http.NewRequestWithContext(
		utils.ContextWithCancelChan(context.Background(), ctx.Done()),
		"GET", signinURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return err
	}
	s.doc = doc
	s.lastResponse = resp
	return nil
}

func (s *authState) doSignin(ctx context.Context) error {
	log.Debug("doSignin")

	// Sign in
	var actionURL string
	var data url.Values
	if form := htmlquery.FindOne(s.doc, "//form[@method]"); form != nil {
		actionURL, data = s.parseForm(form)
	} else {
		return fmt.Errorf("unable to parse form action")
	}
	// add username and password to form values
	data.Set("email", s.c.username)
	data.Set("password", s.c.password)
	// submit form
	resp, err := s.submitForm(ctx, actionURL, data)
	if err != nil {
		return fmt.Errorf("error posting username and password: %s", err)
	}
	defer resp.Body.Close()
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return err
	}
	s.doc = doc
	s.lastResponse = resp
	if msg := s.getMessageBoxString(); msg != "" {
		log.Error(msg)
	}
	return nil
}

func (s *authState) handleCaptcha(ctx context.Context) error {
	log.Debug("handleCaptcha")

	for {
		if img := htmlquery.FindOne(s.doc, "//form//img"); img != nil {
			if a := htmlquery.FindOne(s.doc, "//form//a[text()=\"Try different image\"]"); a == nil {
				return nil
			}
			if msg := s.getMessageBoxString(); msg != "" {
				log.Error(msg)
			}
			if err := s.doCaptchaForm(ctx, img); err != nil {
				return err
			}
		} else {
			return nil
		}
	}
}

func (s *authState) doCaptchaForm(ctx context.Context, img *html.Node) error {
	log.Debug("doCaptchaForm")

	var captcha string
	if s.c.getCaptcha != nil {
		captcha = s.c.getCaptcha(htmlquery.SelectAttr(img, "src"))
	} else {
		return fmt.Errorf("captcha encountered and OptionCaptcha not given")
	}

	var actionURL string
	var data url.Values
	if form := htmlquery.FindOne(s.doc, "//form[@method]"); form != nil {
		actionURL, data = s.parseForm(form)
	} else {
		return fmt.Errorf("unable to parse form action")
	}
	// add username and password to form values
	data.Set("email", s.c.username)
	data.Set("password", s.c.password)
	// add captcha
	data.Set("cvf_captcha_input", captcha)
	// submit form
	resp, err := s.submitForm(ctx, actionURL, data)
	if err != nil {
		return fmt.Errorf("error posting username and password: %s", err)
	}
	defer resp.Body.Close()
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return err
	}
	s.doc = doc
	s.lastResponse = resp
	return nil
}

func (s *authState) handleOTPSelection(ctx context.Context) error {
	log.Debug("handleOTPSelection")

	// Pick OTP method if prompted
	for {
		if form := htmlquery.FindOne(s.doc, "//form[@id = 'auth-select-device-form']"); form != nil {
			if msg := s.getMessageBoxString(); msg != "" {
				log.Error(msg)
			}
			s.doOTPSelectionForm(ctx, form)
		} else {
			return nil
		}
	}
}

type otpOption struct {
	label string
	name  string
	value string
}

func (s *authState) doOTPSelectionForm(ctx context.Context, form *html.Node) error {
	log.Debug("doOTPSelectionForm")

	actionURL, data := s.parseForm(form)

	var options []*otpOption
	var optionLabels []string
	for _, node := range htmlquery.Find(form, "//fieldset/div") {
		input := htmlquery.FindOne(node, "//input[@type='radio']")
		if input == nil {
			continue
		}
		labelText := strings.TrimSpace(htmlquery.InnerText(node))
		options = append(options, &otpOption{
			label: labelText,
			name:  htmlquery.SelectAttr(input, "name"),
			value: htmlquery.SelectAttr(input, "value"),
		})
		optionLabels = append(optionLabels, labelText)
	}
	if len(options) == 0 {
		return errors.New("unable to detect Two-Step verification options")
	}

	option := options[s.c.getChoice("Choose where to receive the One Time Password (OTP)", optionLabels)]
	data.Set(option.name, option.value)
	resp, err := s.submitForm(ctx, actionURL, data)
	if err != nil {
		return fmt.Errorf("error choosing otp method: %w", err)
	}
	defer resp.Body.Close()
	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return err
	}
	s.doc = doc
	s.lastResponse = resp
	return nil
}

func (s *authState) handleOTP(ctx context.Context) error {
	log.Debug("handleOTP")

	// Submit OTP code if promted for one
	for {
		if form := htmlquery.FindOne(s.doc, "//form[@id = 'auth-mfa-form']"); form != nil {
			if msg := s.getMessageBoxString(); msg != "" {
				log.Error(msg)
			}
			s.doOTPForm(ctx, form)
		} else {
			return nil
		}
	}
}

func (s *authState) doOTPForm(ctx context.Context, form *html.Node) error {
	log.Debug("doOTPForm")

	actionURL, data := s.parseForm(form)
	if input := htmlquery.FindOne(form, "//input[@name = 'otpCode']"); input != nil {
		// OTP enabled
		if s.c.getAuthCode == nil {
			return fmt.Errorf("OTP enabled on account and OptionAuthCode not given")
		}
		data.Set("otpCode", s.c.getAuthCode())
		resp, err := s.submitForm(ctx, actionURL, data)
		if err != nil {
			return fmt.Errorf("error posting auth code: %w", err)
		}
		defer resp.Body.Close()
		doc, err := htmlquery.Parse(resp.Body)
		if err != nil {
			return err
		}
		s.doc = doc
		s.lastResponse = resp
	}
	return nil
}

func (s *authState) confirmAuth(ctx context.Context) error {
	log.Debug("confirmAuth")

	req, err := http.NewRequestWithContext(
		utils.ContextWithCancelChan(context.Background(), ctx.Done()),
		"GET", "/lib", nil)
	if err != nil {
		return err
	}
	resp, err := s.c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	s.lastResponse = resp
	if resp.Request.URL.Path == "/ap/signin" {
		return fmt.Errorf("auth verification failed")
	}
	return nil
}
