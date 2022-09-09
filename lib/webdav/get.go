package webdav

import (
	"io"
	"io/ioutil"
	"net/http"
)

func (n *WebDAV) Get(path string) (io.ReadCloser, error) {
	url := n.mkURL(path)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, &Error{Op: http.MethodGet, URL: url, Type: ErrInvalid, Msg: err.Error()}
	}

	if n.AuthFunc != nil {
		n.AuthFunc(req)
	}

	resp, err := n.c.Do(req)
	if err != nil {
		return nil, &Error{Op: http.MethodGet, URL: url, Type: ErrInvalid, Msg: err.Error()}
	}

	rc := readCloser{
		Reader: resp.Body,
		close: func() error {
			io.Copy(ioutil.Discard, resp.Body)
			return resp.Body.Close()
		},
	}

	if resp.StatusCode == http.StatusOK {
		return rc, nil
	}

	rc.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, &Error{Op: http.MethodGet, URL: url, Type: ErrPermission, Msg: resp.Status}

	case http.StatusNotFound:
		return nil, &Error{Op: http.MethodGet, URL: url, Type: ErrNotExist, Msg: resp.Status}

	default:
		return nil, &Error{Op: http.MethodGet, URL: url, Type: ErrInvalid, Msg: resp.Status}
	}
}

type readCloser struct {
	io.Reader
	close func() error
}

func (r readCloser) Close() error {
	return r.close()
}
