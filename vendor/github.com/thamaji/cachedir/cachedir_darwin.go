// +build darwin

package cachedir

import (
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
)

func Dir() (string, error) {
	dir, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "Library", "Caches"), nil
}
