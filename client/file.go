package client

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
)

func init() {
	RegisterFileFetcher(NewHttpFetcher())
}

type FileFetcher interface {
	Accept(url string) bool
	Grab(url string) ([]byte, error)
}

var fetchers []FileFetcher

func RegisterFileFetcher(ff FileFetcher) {
	fetchers = append(fetchers, ff)
}

// FindFileFetcher returns a FileFetcher for this URL or nil if none accept this
// URL.
func FindFileFetcher(url string) FileFetcher {
	for _, ff := range fetchers {
		if ff.Accept(url) {
			return ff
		}
	}
	return nil
}

type HttpFetcher struct {
	re *regexp.Regexp
}

func NewHttpFetcher() FileFetcher {
	re := regexp.MustCompile(`https?://\w+\.[a-z]{2,15}`)
	return &HttpFetcher{re}
}

func (h *HttpFetcher) Accept(url string) bool {
	return h.re.MatchString(url)
}

func (h *HttpFetcher) Grab(url string) ([]byte, error) {
	if !h.Accept(url) {
		return nil, fmt.Errorf("httpfetcher: can't fetch %s", url)
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
