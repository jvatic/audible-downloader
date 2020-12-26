package cookiejar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"

	"github.com/jvatic/audible-downloader/internal/config"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/publicsuffix"
)

// NewJar returns an http.CookieJar that wraps a net/http/cookiejar.Jar for the
// purposes of persisting via a file
func NewJar() (*Jar, error) {
	basejar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	jar := &Jar{
		basejar:       basejar,
		cookies:       make(map[string][]*http.Cookie),
		fileCachePath: filepath.Join(config.Dir(), "cookiejar.json"),
	}

	if err := jar.maybeLoadFileCache(); err != nil {
		log.Errorf("cookiejar: failed to load file cache: %s", err)
	}

	return jar, nil
}

type Jar struct {
	basejar       http.CookieJar
	cookies       map[string][]*http.Cookie
	fileCachePath string
}

func urlKey(u *url.URL) string {
	return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
}

func (j *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	log.Debugf("cookiejar.SetCookies(%s, %d)", u.Hostname(), len(cookies))
	j.basejar.SetCookies(u, cookies)
	cookies = j.basejar.Cookies(u)
	j.cookies[urlKey(u)] = cookies
	if err := j.maybeSaveFileCache(); err != nil {
		log.Errorf("cookiejar: failed to save file cache: %s", err)
	}
}

func (j *Jar) Cookies(u *url.URL) []*http.Cookie {
	cookies := j.basejar.Cookies(u)
	log.Debugf("cookiejar.Cookies(%s) -> %d", u.Hostname(), len(cookies))
	return cookies
}

func (j *Jar) maybeLoadFileCache() error {
	if j.fileCachePath == "" {
		return nil
	}

	file, err := os.OpenFile(j.fileCachePath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer file.Close()

	var cookies map[string][]*http.Cookie
	if err := json.NewDecoder(file).Decode(&cookies); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	j.cookies = cookies

	// load cookies into basejar
	for ustr, c := range cookies {
		if u, err := url.Parse(ustr); err == nil {
			j.basejar.SetCookies(u, c)
		}
	}

	return nil
}

func (j *Jar) maybeSaveFileCache() error {
	if j.fileCachePath == "" {
		return nil
	}

	file, err := os.OpenFile(j.fileCachePath, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(j.cookies); err != nil {
		return err
	}

	return nil
}
