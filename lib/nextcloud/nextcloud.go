package nextcloud

import (
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/kurusugawa-computer/nextcloud-cli/lib/webdav"
)

func New(url string, httpClient *http.Client, authFunc webdav.AuthFunc) *Nextcloud {
	nextcloud := Nextcloud{
		URL: url,
		w:   webdav.New(url, httpClient, authFunc),
	}

	return &nextcloud
}

type Nextcloud struct {
	URL string
	w   *webdav.WebDAV
}

func (n *Nextcloud) Stat(path string) (os.FileInfo, error) {
	path = url.PathEscape(path)
	responses, err := n.w.Propfind(path, webdav.Depth0, propfind)
	if err != nil {
		return nil, &os.PathError{Op: "Stat", Path: path, Err: webdavError(err)}
	}

	if len(responses) != 1 {
		if len(responses) <= 0 {
			return nil, &os.PathError{Op: "Stat", Path: path, Err: os.ErrNotExist}
		}
		return nil, &os.PathError{Op: "Stat", Path: path, Err: os.ErrInvalid}
	}

	fi, err := fileInfo(responses[0])
	if err != nil {
		return nil, &os.PathError{Op: "Stat", Path: path, Err: os.ErrInvalid}
	}

	return fi, nil
}

func (n *Nextcloud) ReadFile(path string) (io.ReadCloser, error) {
	path = url.PathEscape(path)
	body, err := n.w.Get(path)
	if err != nil {
		return nil, &os.PathError{Op: "ReadFile", Path: path, Err: webdavError(err)}
	}

	return body, nil
}

func (n *Nextcloud) WriteFile(path string, body io.Reader) error {
	path = url.PathEscape(path)
	if err := n.w.Put(path, body); err != nil {
		return &os.PathError{Op: "WriteFile", Path: path, Err: webdavError(err)}
	}

	return nil
}

func (n *Nextcloud) ReadDir(path string) ([]os.FileInfo, error) {
	path = url.PathEscape(path)
	responses, err := n.w.Propfind(path, webdav.Depth1, propfind)
	if err != nil {
		return nil, &os.PathError{Op: "ReadDir", Path: path, Err: webdavError(err)}
	}

	// TODO: もう少しマシな、自身を除く処理
	responses = responses[1:]

	fl := make([]os.FileInfo, 0, len(responses))
	for _, response := range responses {
		fi, err := fileInfo(response)
		if err != nil {
			return nil, &os.PathError{Op: "ReadDir", Path: path, Err: os.ErrInvalid}
		}

		fl = append(fl, fi)
	}

	return fl, nil
}

func (n *Nextcloud) Mkdir(path string) error {
	path = url.PathEscape(path)
	if err := n.w.Mkcol(path); err != nil {
		return &os.PathError{Op: "Mkdir", Path: path, Err: webdavError(err)}
	}

	return nil
}

func (n *Nextcloud) MkdirAll(path string) error {
	fi, err := n.Stat(path)
	if err == nil {
		if fi.IsDir() {
			return nil
		}
		return &os.PathError{Op: "MkdirAll", Path: path, Err: os.ErrExist}
	}

	i := len(path)
	for i > 0 && path[i-1] == '/' {
		i--
	}

	j := i
	for j > 0 && path[j-1] != '/' {
		j--
	}

	if j > 1 {
		err = n.MkdirAll(path[0 : j-1])
		if err != nil {
			return err
		}
	}

	err = n.Mkdir(path)
	if err != nil {
		dir, err1 := n.Stat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}

	return nil
}

func (n *Nextcloud) Delete(path string) error {
	path = url.PathEscape(path)
	if err := n.w.Delete(path); err != nil {
		return &os.PathError{Op: "Delete", Path: path, Err: webdavError(err)}
	}
	return nil
}
