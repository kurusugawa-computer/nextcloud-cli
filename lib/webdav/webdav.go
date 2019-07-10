package webdav

import (
	"net/http"
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

func BasicAuth(username, password string) AuthFunc {
	return func(r *http.Request) {
		r.SetBasicAuth(username, password)
	}
}

type AuthFunc func(*http.Request)
