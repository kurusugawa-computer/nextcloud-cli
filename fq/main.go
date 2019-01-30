package fq

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func ParseFiles(args ...string) ([]string, []string) {
	for i := range args {
		if strings.HasPrefix(args[i], "-") || args[i] == "(" {
			return args[:i], args[i:]
		}
	}
	return args, nil
}

func main() {
	args := os.Args[1:]

	files, args := ParseFiles(args...)

	expr, err := Parse(args...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	for _, path := range files {
		file, err := os.Stat(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}

		if err := do(path, file, expr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
	}
}

func do(path string, file os.FileInfo, expr Expr) error {
	if !file.IsDir() {
		result, err := expr.Apply(path, file)
		if err != nil {
			return err
		}

		if result {
			fmt.Println(path)
		}

		return nil
	}

	fl, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, f := range fl {
		if err := do(filepath.Join(path, f.Name()), f, expr); err != nil {
			return err
		}
	}

	return nil
}
