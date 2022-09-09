package webdav

import (
	"io"
	"io/ioutil"
	"net/http"
)

func (n *WebDAV) Put(path string, body io.Reader) error {
	url := n.mkURL(path)
	req, err := http.NewRequest(http.MethodPut, url, body)
	if err != nil {
		return &Error{Op: http.MethodPut, URL: url, Type: ErrInvalid, Msg: err.Error()}
	}

	if n.AuthFunc != nil {
		n.AuthFunc(req)
	}

	resp, err := n.c.Do(req)
	if err != nil {
		return &Error{Op: http.MethodPut, URL: url, Type: ErrInvalid, Msg: err.Error()}
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil

	case http.StatusUnauthorized, http.StatusForbidden:
		return &Error{Op: http.MethodPut, URL: url, Type: ErrPermission, Msg: resp.Status}

	case http.StatusConflict, http.StatusNotFound:
		return &Error{Op: http.MethodPut, URL: url, Type: ErrNotExist, Msg: resp.Status}

	default:
		return &Error{Op: http.MethodPut, URL: url, Type: ErrInvalid, Msg: resp.Status}
	}
}
