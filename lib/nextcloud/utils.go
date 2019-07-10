package nextcloud

import (
	"os"

	"github.com/kurusugawa-computer/nextcloud-cli/lib/webdav"
)

func webdavError(err error) error {
	switch webdav.TypeOf(err) {
	case webdav.ErrInvalid:
		return os.ErrInvalid

	case webdav.ErrPermission:
		return os.ErrPermission

	case webdav.ErrExist:
		return os.ErrExist

	case webdav.ErrNotExist:
		return os.ErrNotExist

	default:
		return os.ErrInvalid
	}
}
