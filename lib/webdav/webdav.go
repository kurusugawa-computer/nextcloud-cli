package webdav

import (
	"net/http"
	"net/url"
	_path "path"
	"strings"
)

func New(url string, httpClient *http.Client, authFunc AuthFunc) *WebDAV {
	nextcloud := WebDAV{
		URL:      url,
		AuthFunc: authFunc,
		c:        httpClient,
	}

	if nextcloud.c == nil {
		nextcloud.c = http.DefaultClient
	}

	return &nextcloud
}

type WebDAV struct {
	URL      string
	AuthFunc AuthFunc
	c        *http.Client
}

func (n *WebDAV) mkURL(path string) string {
	return strings.TrimSuffix(n.URL, "/") + "/" + url.PathEscape(strings.TrimPrefix(_path.Clean(path), "/"))
}

func BasicAuth(username, password string) AuthFunc {
	return func(r *http.Request) {
		r.SetBasicAuth(username, password)
	}
}

type AuthFunc func(*http.Request)
