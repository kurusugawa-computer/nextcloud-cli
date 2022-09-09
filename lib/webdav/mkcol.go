package webdav

import (
	"io"
	"io/ioutil"
	"net/http"
)

func (n *WebDAV) Mkcol(path string) error {
	const MethodMkcol = "MKCOL"

	url := n.mkURL(path)
	req, err := http.NewRequest(MethodMkcol, url, nil)
	if err != nil {
		return &Error{Op: MethodMkcol, URL: url, Type: ErrInvalid, Msg: err.Error()}
	}

	if n.AuthFunc != nil {
		n.AuthFunc(req)
	}

	resp, err := n.c.Do(req)
	if err != nil {
		return &Error{Op: MethodMkcol, URL: url, Type: ErrInvalid, Msg: err.Error()}
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusCreated:
		return nil

	case http.StatusUnauthorized, http.StatusForbidden:
		return &Error{Op: MethodMkcol, URL: url, Type: ErrPermission, Msg: resp.Status}

	case http.StatusMethodNotAllowed:
		return &Error{Op: MethodMkcol, URL: url, Type: ErrExist, Msg: resp.Status}

	case http.StatusConflict:
		return &Error{Op: MethodMkcol, URL: url, Type: ErrNotExist, Msg: resp.Status}

	case http.StatusUnsupportedMediaType, http.StatusInsufficientStorage:
		return &Error{Op: MethodMkcol, URL: url, Type: ErrInvalid, Msg: resp.Status}

	default:
		return &Error{Op: MethodMkcol, URL: url, Type: ErrInvalid, Msg: resp.Status}
	}
}
