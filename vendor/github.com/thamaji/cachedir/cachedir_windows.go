// +build windows

package cachedir

import (
	"os"
	"path/filepath"
)

func Dir() (string, error) {
	return filepath.Join(filepath.FromSlash(os.Getenv("LOCALAPPDATA")), "cache"), nil
}
