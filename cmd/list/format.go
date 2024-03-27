package list

import (
	"fmt"
	"math"
	"os"
	_path "path"
	"strconv"
	"strings"
	"time"

	lscolors "github.com/kurusugawa-computer/ls-colors-go"
	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
)

type entry struct {
	Path string
	os.FileInfo
}

func FormatFileInfo(fi os.FileInfo, color *lscolors.LSColors) ([]string, error) {
	var err error = nil
	mode, err := formatMode(color, fi.Mode().String())
	if err != nil {
		return nil, err
	}

	var size string
	if fi.Size() > 0 {
		size, err = formatSize(color, fi.Size())
		if err != nil {
			return nil, err
		}
	} else {
		size = "-"
		if c, ok := color.Unknowns["xx"]; ok {
			size, err = applyColor(color, c[0], size)
			if err != nil {
				return nil, err
			}
		}
	}

	owner := ""
	if fi, ok := fi.(*nextcloud.FileInfo); ok {
		owner = fi.OwnerDisplayName()
		owner = strings.TrimSuffix(owner, ")")
		if i := strings.LastIndex(owner, "("); i >= 0 {
			owner = strings.TrimSpace(owner[:i])
		}
		if c, ok := color.Unknowns["uu"]; ok {
			owner, err = applyColor(color, c[0], owner)
			if err != nil {
				return nil, err
			}
		}
	}

	modTime := fi.ModTime().In(time.Local).Format("2006-01-02 15:04")
	if c, ok := color.Unknowns["da"]; ok {
		modTime, err = applyColor(color, c[0], modTime)
		if err != nil {
			return nil, err
		}
	}

	name := fi.Name()
	if fi.IsDir() {
		name = strings.TrimSuffix(name, "/")
		if color.Directory != nil {
			name, err = applyColor(color, *color.Directory, name)
			if err != nil {
				return nil, err
			}
		}
	}

	if entry, ok := fi.(entry); ok {
		dir := _path.Dir(entry.Path) + "/"
		if c, ok := color.Unknowns["lp"]; ok {
			dir, err = applyColor(color, c[0], dir)
			if err != nil {
				return nil, err
			}
		}
		name = dir + name
	}

	return []string{
		mode,
		size,
		owner,
		modTime,
		name,
	}, nil
}

func unwrapString(s *string) string {
	if s == nil {
		return ""
	} else {
		return *s
	}
}

func applyColor(color *lscolors.LSColors, indicator string, text string) (string, error) {
	dst := strings.Builder{}
	if _, err := dst.WriteString(unwrapString(color.LeftOfColorSequence)); err != nil {
		return "", err
	}
	if _, err := dst.WriteString(indicator); err != nil {
		return "", err
	}
	if _, err := dst.WriteString(unwrapString(color.RightOfColorSequence)); err != nil {
		return "", err
	}
	if _, err := dst.WriteString(text); err != nil {
		return "", err
	}
	if _, err := dst.WriteString(unwrapString(color.LeftOfColorSequence)); err != nil {
		return "", err
	}
	if _, err := dst.WriteString(unwrapString(color.ResetOrdinaryColor)); err != nil {
		return "", err
	}
	if _, err := dst.WriteString(unwrapString(color.RightOfColorSequence)); err != nil {
		return "", err
	}
	return dst.String(), nil
}

func formatMode(color *lscolors.LSColors, mode string) (string, error) {
	colors := make([]*string, 10)
	var punctuationColor *string = nil
	if len(color.Unknowns["xx"]) > 0 {
		punctuationColor = &color.Unknowns["xx"][0]
	}
	indicators := []string{"", "ur", "uw", "ux", "gr", "gw", "gx", "tr", "tw", "tx"}
	for i, ind := range indicators {
		if c, ok := color.Unknowns[ind]; ok {
			colors[i] = &c[0]
		}
	}
	buf := strings.Builder{}
	buf.Grow(10)
	if mode[0] == 'd' {
		if color.Directory != nil {
			text, err := applyColor(color, *color.Directory, "d")
			if err != nil {
				return "", err
			}
			buf.WriteString(text)
		} else {
			buf.WriteByte('d')
		}
	} else {
		if color.FileDefault != nil {
			text, err := applyColor(color, *color.FileDefault, mode[0:1])
			if err != nil {
				return "", err
			}
			buf.WriteString(text)
		} else {
			buf.WriteByte(mode[0])
		}
	}
	for i := range mode[1:] {
		if mode[i+1] == '-' {
			if punctuationColor != nil {
				text, err := applyColor(color, *punctuationColor, mode[i+1:i+2])
				if err != nil {
					return "", err
				}
				buf.WriteString(text)
			} else {
				buf.WriteByte(mode[i+1])
			}
		} else {
			if colors[i+1] != nil {
				text, err := applyColor(color, *colors[i+1], mode[i+1:i+2])
				if err != nil {
					return "", err
				}
				buf.WriteString(text)
			} else {
				buf.WriteByte(mode[i+1])
			}
		}
	}
	return buf.String(), nil
}

func formatSize(color *lscolors.LSColors, size int64) (string, error) {
	var err error
	var numberDefault *string = nil
	numberIndicators := []string{"nb", "nk", "nm", "ng", "nt"}
	numberColors := make([]*string, len(numberIndicators))
	var unitDefault *string = nil
	unitIndicators := []string{"ub", "uk", "um", "ug", "ut"}
	unitColors := make([]*string, len(unitIndicators))
	units := []string{"", "k", "M", "G", "T", "P", "E"}

	if c, ok := color.Unknowns["sn"]; ok {
		numberDefault = &c[0]
	}
	if c, ok := color.Unknowns["sb"]; ok {
		unitDefault = &c[0]
	}
	for i := range numberIndicators {
		if c, ok := color.Unknowns[numberIndicators[i]]; ok {
			numberColors[i] = &c[i]
		}
	}
	for i := range unitIndicators {
		if c, ok := color.Unknowns[unitIndicators[i]]; ok {
			unitColors[i] = &c[i]
		}
	}

	if size < 10 {
		text := strconv.FormatInt(size, 10)
		if numberColors[0] != nil {
			text, err = applyColor(color, *numberColors[0], text)
			if err != nil {
				return "", err
			}
		}
		return text, nil
	}

	e := math.Floor(math.Log(float64(size)) / math.Log(1000))
	sizeIndex := int(e)

	v := math.Floor(float64(size)/math.Pow(1000, e)*10+0.5) / 10

	format := "%.0f"
	if v < 10 {
		format = "%.1f"
	}
	num := fmt.Sprintf(format, v)
	numIndex := len(numberColors) - 1
	if sizeIndex < numIndex {
		numIndex = sizeIndex
	}
	if numberColors[numIndex] != nil {
		num, err = applyColor(color, *numberColors[numIndex], num)
		if err != nil {
			return "", err
		}
	} else if numberDefault != nil {
		num, err = applyColor(color, *numberDefault, num)
		if err != nil {
			return "", err
		}
	}

	unit := units[sizeIndex]
	unitIndex := len(unitColors) - 1
	if sizeIndex < unitIndex {
		unitIndex = sizeIndex
	}
	if unitColors[unitIndex] != nil {
		unit, err = applyColor(color, *unitColors[unitIndex], unit)
		if err != nil {
			return "", nil
		}
	} else if unitDefault != nil {
		unit, err = applyColor(color, *unitDefault, unit)
		if err != nil {
			return "", err
		}
	}
	return num + unit, nil
}
