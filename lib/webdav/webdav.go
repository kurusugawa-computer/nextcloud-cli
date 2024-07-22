package webdav

import (
	"fmt"
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

func BasicAuth(username, password, appname, version string) AuthFunc {
	return func(r *http.Request) {
		r.SetBasicAuth(username, password)

		userAgent := fmt.Sprintf("%s/%s", appname, version)
		r.Header.Set("User-Agent", userAgent)
	}
}

type AuthFunc func(*http.Request)
