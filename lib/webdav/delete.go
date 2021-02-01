package webdav

import (
	"io"
	"io/ioutil"
	"net/http"
	_path "path"
	"strings"
)

func (n *WebDAV) Delete(path string) error {
	path = strings.TrimSuffix(n.URL, "/") +
		"/" +
		strings.TrimPrefix(_path.Clean(path), "/")
	req, err := http.NewRequest(http.MethodDelete, path, nil)
	if err != nil {
		return &Error{
			Op:   http.MethodDelete,
			Path: path,
			Type: ErrInvalid,
			Msg:  err.Error(),
		}
	}
	if n.AuthFunc != nil {
		n.AuthFunc(req)
	}
	res, err := n.c.Do(req)
	if err != nil {
		return &Error{
			Op:   http.MethodDelete,
			Path: path,
			Type: ErrInvalid,
			Msg:  err.Error(),
		}
	}

	defer func() {
		io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
	}()

	switch res.StatusCode {
	case http.StatusOK, http.StatusNoContent:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return &Error{
			Op:   http.MethodDelete,
			Path: path,
			Type: ErrPermission,
			Msg:  res.Status,
		}
	case http.StatusNotFound:
		return &Error{
			Op:   http.MethodDelete,
			Path: path,
			Type: ErrNotExist,
			Msg:  res.Status,
		}
	default:
		return &Error{
			Op:   http.MethodDelete,
			Path: path,
			Type: ErrInvalid,
			Msg:  res.Status,
		}
	}
}
