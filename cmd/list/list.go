package list

import (
	"fmt"
	"os"
	_path "path"
	"sort"

	lscolors "github.com/kurusugawa-computer/ls-colors-go"
	"github.com/kurusugawa-computer/nextcloud-cli/lib/nextcloud"
	"github.com/thamaji/tablewriter"
	"github.com/thamaji/wordwriter"
	"golang.org/x/crypto/ssh/terminal"
)

type ctx struct {
	isTerminal bool
}

func Do(n *nextcloud.Nextcloud, long bool, paths ...string) error {
	files := []entry{}
	dirs := []entry{}

	isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))

	var color *lscolors.LSColors = nil
	if isTerminal {
		config := os.Getenv("LS_COLORS") + os.Getenv("EXA_COLORS") + ":" + os.Getenv("EZA_COLORS") + ":"
		var err error
		color, err = lscolors.ParseLS_COLORS(config, true)
		if err != nil {
			return err
		}
	}

	for _, path := range paths {
		fi, err := n.Stat(path)
		if err != nil {
			return err
		}

		entry := entry{
			Path:     _path.Clean(path),
			FileInfo: fi,
		}

		if !fi.IsDir() {
			files = append(files, entry)
		} else {
			dirs = append(dirs, entry)
		}
	}

	switch {
	case isTerminal && long:
		writer := tablewriter.New(os.Stdout)
		writer.SetAligns(tablewriter.AlignLeft, tablewriter.AlignRight, tablewriter.AlignLeft, tablewriter.AlignLeft)
		for _, entry := range files {
			texts, err := FormatFileInfo(entry, color)
			if err != nil {
				return err
			}
			writer.Add(texts...)
		}
		writer.Flush()

		for i, entry := range dirs {
			if len(files) > 0 {
				fmt.Println()
				fmt.Println(entry.Path + ":")
			} else if len(dirs) > 1 {
				if i > 0 {
					fmt.Println()
				}
				fmt.Println(entry.Path + ":")
			}

			writer := tablewriter.New(os.Stdout)
			writer.SetAligns(tablewriter.AlignLeft, tablewriter.AlignRight, tablewriter.AlignLeft, tablewriter.AlignLeft)

			fl, err := n.ReadDir(entry.Path)
			if err != nil {
				return err
			}

			sort.Slice(fl, func(i, j int) bool {
				return fl[i].Name() < fl[j].Name()
			})

			for _, fi := range fl {
				texts, err := FormatFileInfo(fi, color)
				if err != nil {
					return err
				}
				writer.Add(texts...)
			}
			writer.Flush()
		}

	case isTerminal && !long:
		writer := wordwriter.New(os.Stdout)
		for _, file := range files {
			writer.Add(file.Name())
		}
		writer.Flush()

		for i, entry := range dirs {
			if len(files) > 0 {
				fmt.Println()
				fmt.Println(entry.Path + ":")
			} else if len(dirs) > 1 {
				if i > 0 {
					fmt.Println()
				}
				fmt.Println(entry.Path + ":")
			}

			fl, err := n.ReadDir(entry.Path)
			if err != nil {
				return err
			}

			sort.Slice(fl, func(i, j int) bool {
				return fl[i].Name() < fl[j].Name()
			})

			writer := wordwriter.New(os.Stdout)
			for _, fi := range fl {
				name := fi.Name()
				if fi.IsDir() && color.Directory != nil {
					name, err = applyColor(color, *color.Directory, name)
					if err != nil {
						return err
					}
				}
				writer.Add(name)
			}
			writer.Flush()
		}

	case !isTerminal:
		for _, entry := range files {
			fmt.Println(entry.Path)
		}

		for i, entry := range dirs {
			if len(files) > 0 {
				fmt.Println()
				fmt.Println(entry.Path + ":")
			} else if len(dirs) > 1 {
				if i > 0 {
					fmt.Println()
				}
				fmt.Println(entry.Path + ":")
			}

			fl, err := n.ReadDir(entry.Path)
			if err != nil {
				return err
			}

			sort.Slice(fl, func(i, j int) bool {
				return fl[i].Name() < fl[j].Name()
			})

			for _, fi := range fl {
				fmt.Println(fi.Name())
			}
		}
	}

	return nil
}
