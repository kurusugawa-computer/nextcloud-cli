package list

import (
	"fmt"
	"math"
	"os"
	_path "path"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
)

type entry struct {
	Path string
	os.FileInfo
}

func FormatFileInfo(fi os.FileInfo) []string {
	mode := fi.Mode().String()
	mode = func(m string) string {
		buf := &strings.Builder{}

		colors := []*color.Color{
			color.New(color.FgBlue),
			color.New(color.FgHiYellow, color.Bold),
			color.New(color.FgHiRed, color.Bold),
			color.New(color.FgGreen, color.Bold),
			color.New(color.FgHiRed),
			color.New(color.FgRed),
			color.New(color.FgGreen),
			color.New(color.FgHiRed),
			color.New(color.FgRed),
			color.New(color.FgGreen),
		}
		for i := range m {
			c := color.New(color.FgHiBlack)
			if m[i] != '-' {
				c = colors[i]
			}
			fmt.Fprint(buf, c.Sprint(string(m[i])))
		}

		return buf.String()
	}(mode)

	var size, unit string
	if fi.Size() > 0 {
		size, unit = formatSize(fi.Size())
		size = color.New(color.FgGreen, color.Bold).Sprint(size)
		unit = color.New(color.FgGreen).Sprint(unit)
	} else {
		size, unit = "", "-"
		unit = color.New(color.FgHiBlack).Sprint(unit)
	}

	owner := ""
	if fi, ok := fi.(*nextcloud.FileInfo); ok {
		owner = fi.OwnerDisplayName()
		owner = strings.TrimSuffix(owner, ")")
		if i := strings.LastIndex(owner, "("); i >= 0 {
			owner = strings.TrimSpace(owner[:i])
		}

		owner = color.New(color.FgHiYellow, color.Bold).Sprint(owner)
	}

	modTime := fi.ModTime().In(time.Local).Format("2006-01-02 15:04")
	modTime = color.New(color.FgBlue).Sprint(modTime)

	name := fi.Name()
	if fi.IsDir() {
		name = strings.TrimSuffix(name, "/")
		name = color.New(color.FgBlue).Sprint(name)
		name += "/"
	}

	if entry, ok := fi.(entry); ok {
		dir := _path.Dir(entry.Path) + "/"
		dir = color.New(color.FgCyan).Sprint(dir)
		name = dir + name
	}

	return []string{
		mode,
		size + unit,
		owner,
		modTime,
		name,
	}
}

func formatSize(size int64) (string, string) {
	if size < 10 {
		return strconv.FormatInt(size, 10), ""
	}

	e := math.Floor(math.Log(float64(size)) / math.Log(1000))
	unit := []string{"", "k", "M", "G", "T", "P", "E"}[int(e)]

	v := math.Floor(float64(size)/math.Pow(1000, e)*10+0.5) / 10

	format := "%.0f"
	if v < 10 {
		format = "%.1f"
	}
	return fmt.Sprintf(format, v), unit
}
