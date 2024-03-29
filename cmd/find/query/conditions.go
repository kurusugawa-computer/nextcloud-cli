package query

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var Conditions = map[string]Parser{
	"-name": ParserFunc(func(scope *Scope) (Expr, error) {
		arg, ok := scope.Next()
		if !ok {
			return nil, errors.New("条件式 -name には引数が必要です。")
		}

		if _, err := path.Match(arg, ""); err != nil {
			return nil, errors.New("invalid pattern: " + arg)
		}

		expr := ExprFunc(func(p string, file os.FileInfo) (bool, error) {
			return path.Match(arg, file.Name())
		})

		return expr, nil
	}),
	"-iname": ParserFunc(func(scope *Scope) (Expr, error) {
		arg, ok := scope.Next()
		if !ok {
			return nil, errors.New("条件式 -iname には引数が必要です。")
		}

		arg = strings.ToLower(arg)

		if _, err := path.Match(arg, ""); err != nil {
			return nil, errors.New("invalid pattern: " + arg)
		}

		expr := ExprFunc(func(p string, file os.FileInfo) (bool, error) {
			return path.Match(arg, strings.ToLower(file.Name()))
		})

		return expr, nil
	}),
	"-path": ParserFunc(func(scope *Scope) (Expr, error) {
		arg, ok := scope.Next()
		if !ok {
			return nil, errors.New("条件式 -path には引数が必要です。")
		}

		if _, err := path.Match(arg, ""); err != nil {
			return nil, errors.New("invalid pattern: " + arg)
		}

		expr := ExprFunc(func(p string, file os.FileInfo) (bool, error) {
			return path.Match(arg, p)
		})

		return expr, nil
	}),
	"-ipath": ParserFunc(func(scope *Scope) (Expr, error) {
		arg, ok := scope.Next()
		if !ok {
			return nil, errors.New("条件式 -ipath には引数が必要です。")
		}

		arg = strings.ToLower(arg)

		if _, err := path.Match(arg, ""); err != nil {
			return nil, errors.New("invalid pattern: " + arg)
		}

		expr := ExprFunc(func(p string, file os.FileInfo) (bool, error) {
			return path.Match(arg, p)
		})

		return expr, nil
	}),
	"-regex": ParserFunc(func(scope *Scope) (Expr, error) {
		arg, ok := scope.Next()
		if !ok {
			return nil, errors.New("条件式 -regex には引数が必要です。")
		}

		regexp, err := regexp.Compile(arg)
		if err != nil {
			return nil, errors.New("failed to parse regexp '" + arg + "': " + err.Error())
		}

		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			return regexp.MatchString(path), nil
		})

		return expr, nil
	}),
	"-mtime": ParserFunc(func(scope *Scope) (Expr, error) {
		arg, ok := scope.Next()
		if !ok {
			return nil, errors.New("条件式 -mtime には引数が必要です。")
		}

		var op func(time.Time, time.Time, int64) bool

		switch arg[0] {
		case '+':
			op = func(a, b time.Time, days int64) bool {
				return a.Sub(b) > time.Duration(days)*24*time.Hour
			}
			arg = arg[1:]
		case '-':
			op = func(a, b time.Time, days int64) bool {
				return b.Sub(a) > time.Duration(days)*24*time.Hour
			}
			arg = arg[1:]
		default:
			op = func(a, b time.Time, days int64) bool {
				return int64(a.Sub(b).Hours()/24) == days
			}
		}

		days, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return nil, errors.New("failed to parse number '" + arg + "':" + err.Error())
		}

		now := time.Now()

		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			return op(file.ModTime(), now, days), nil
		})

		return expr, nil
	}),
	"-newer": ParserFunc(func(scope *Scope) (Expr, error) {
		arg, ok := scope.Next()
		if !ok {
			return nil, errors.New("条件式 -newer には引数が必要です。")
		}

		stat, err := os.Stat(arg)
		if err != nil {
			return nil, errors.New("failed to get modTime '" + arg + "':" + err.Error())
		}

		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			return file.ModTime().Sub(stat.ModTime()) > 0, nil
		})

		return expr, nil
	}),
	"-newermt": ParserFunc(func(scope *Scope) (Expr, error) {
		arg, ok := scope.Next()
		if !ok {
			return nil, errors.New("条件式 -newermt には引数が必要です。")
		}

		modTime, err := time.Parse("2006-01-02 15:04:05", arg)
		if err != nil {
			modTime, err = time.Parse("2006-01-02", arg)
			if err != nil {
				return nil, errors.New("failed to parse time '" + arg + "':" + err.Error())
			}
		}

		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			return file.ModTime().Sub(modTime) > 0, nil
		})

		return expr, nil
	}),
	"-size": ParserFunc(func(scope *Scope) (Expr, error) {
		arg, ok := scope.Next()
		if !ok {
			return nil, errors.New("条件式 -size には引数が必要です。")
		}

		var op func(int64, int64) bool

		switch arg[0] {
		case '+':
			op = func(a, b int64) bool {
				return a < b
			}
			arg = arg[1:]
		case '-':
			op = func(a, b int64) bool {
				return a > b
			}
			arg = arg[1:]
		default:
			op = func(a, b int64) bool {
				return a == b
			}
		}

		var scale int64

		switch arg[len(arg)-1] {
		case 'k':
			scale = 1024
			arg = arg[:len(arg)-1]
		case 'M':
			scale = 1024 * 1024
			arg = arg[:len(arg)-1]
		case 'G':
			scale = 1024 * 1024 * 1024
			arg = arg[:len(arg)-1]
		default:
			scale = 1
		}

		size, err := strconv.ParseInt(arg, 10, 64)
		if err != nil {
			return nil, errors.New("failed to parse size '" + arg + "':" + err.Error())
		}

		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			return op(file.Size(), size*scale), nil
		})

		return expr, nil
	}),
	"-empty": ParserFunc(func(scope *Scope) (Expr, error) {
		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			// TODO: 空のディレクトリを判定する材料がここにはない
			if file.IsDir() {
				return false, nil
			}

			return file.Size() == 0, nil
		})

		return expr, nil
	}),
	"-type": ParserFunc(func(scope *Scope) (Expr, error) {
		arg, ok := scope.Next()
		if !ok {
			return nil, errors.New("条件式 -type には引数が必要です。")
		}

		var op func(os.FileInfo) bool
		switch arg {
		case "d":
			op = func(file os.FileInfo) bool {
				return file.IsDir()
			}
		case "f":
			op = func(file os.FileInfo) bool {
				return file.Mode().IsRegular()
			}
		default:
			return nil, errors.New("invalid type '" + arg)
		}

		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			return op(file), nil
		})

		return expr, nil
	}),
	"-true": ParserFunc(func(scope *Scope) (Expr, error) {
		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			return true, nil
		})

		return expr, nil
	}),
	"-false": ParserFunc(func(scope *Scope) (Expr, error) {
		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			return false, nil
		})

		return expr, nil
	}),
	"-ls": ParserFunc(func(scope *Scope) (Expr, error) {
		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			var mtime string
			if file.ModTime().Local().Before(time.Now().Local().AddDate(0, -6, 0)) {
				mtime = file.ModTime().Local().Format("Jan 02 2006")
			} else {
				mtime = file.ModTime().Local().Format("Jan 02 03:04")
			}
			// TODO: We should print the owner of the file
			fmt.Printf("%v %8d %v %v\n", file.Mode(), file.Size(), mtime, file.Name())
			return true, nil
		})

		scope.noDefaultPrint = true

		return expr, nil
	}),
	"-print": ParserFunc(func(scope *Scope) (Expr, error) {
		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			fmt.Println(path)
			return true, nil
		})

		scope.noDefaultPrint = true

		return expr, nil
	}),
	"-print0": ParserFunc(func(scope *Scope) (Expr, error) {
		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			fmt.Printf("%s\x00", path)
			return true, nil
		})

		scope.noDefaultPrint = true

		return expr, nil
	}),
	"-quit": ParserFunc(func(scope *Scope) (Expr, error) {
		expr := ExprFunc(func(path string, file os.FileInfo) (bool, error) {
			os.Exit(0)
			return true, nil
		})

		return expr, nil
	}),
}
