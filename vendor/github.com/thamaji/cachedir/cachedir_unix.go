// +build !darwin
// +build !windows

package cachedir

import (
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
)

func Dir() (string, error) {
	// TODO
	// if runtime.GOOS == "android" {
	// } else {

	dir := os.Getenv("XDG_CACHE_HOME")
	if dir != "" {
		return dir, nil
	}

	dir, err := homedir.Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, ".cache"), nil
}
