package webdav

import (
	"io"
	"io/ioutil"
	"net/http"
	_path "path"
	"strings"
)

func (n *WebDAV) Put(path string, body io.Reader) error {
	path = strings.TrimSuffix(n.URL, "/") + "/" + strings.TrimPrefix(_path.Clean(path), "/")
	req, err := http.NewRequest(http.MethodPut, path, body)
	if err != nil {
		return &Error{Op: http.MethodPut, Path: path, Type: ErrInvalid, Msg: err.Error()}
	}

	if n.AuthFunc != nil {
		n.AuthFunc(req)
	}

	resp, err := n.c.Do(req)
	if err != nil {
		return &Error{Op: http.MethodPut, Path: path, Type: ErrInvalid, Msg: err.Error()}
	}
	defer func() {
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated, http.StatusNoContent:
		return nil

	case http.StatusUnauthorized, http.StatusForbidden:
		return &Error{Op: http.MethodPut, Path: path, Type: ErrPermission, Msg: resp.Status}

	case http.StatusConflict, http.StatusNotFound:
		return &Error{Op: http.MethodPut, Path: path, Type: ErrNotExist, Msg: resp.Status}

	default:
		return &Error{Op: http.MethodPut, Path: path, Type: ErrInvalid, Msg: resp.Status}
	}
}
