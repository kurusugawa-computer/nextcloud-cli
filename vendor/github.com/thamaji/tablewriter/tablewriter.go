package tablewriter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

func New(output io.Writer) *TableWriter {
	return &TableWriter{
		output: output,
	}
}

type TableWriter struct {
	output io.Writer
	max    []int
	aligns []int
	rows   [][]string
}

var ansi = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

const (
	AlignLeft = 1 + iota
	AlignRight
	AlignCenter
)

func (w *TableWriter) SetAligns(aligns ...int) {
	w.aligns = aligns
}

func (w *TableWriter) Add(row ...string) {
	if len(w.max) < len(row) {
		w.max = append(w.max, make([]int, len(row)-len(w.max))...)
	}

	for i := range w.max {
		if i >= len(row) {
			break
		}

		size := runewidth.StringWidth(ansi.ReplaceAllLiteralString(row[i], ""))

		if w.max[i] < size {
			w.max[i] = size
		}
	}

	w.rows = append(w.rows, row)
}

func (w *TableWriter) Flush() {
	buf := bytes.NewBuffer([]byte{})

	output := w.output
	if output == nil {
		output = os.Stdout
	}

	for _, row := range w.rows {
		buf.Reset()

		for i := range row {
			size := runewidth.StringWidth(ansi.ReplaceAllLiteralString(row[i], ""))

			align := 0
			if i < len(w.aligns) {
				align = w.aligns[i]
			}

			var left, right int

			switch align {
			case AlignLeft:
				left = 0
				right = w.max[i] - size

			case AlignRight:
				left = w.max[i] - size
				right = 0

			case AlignCenter:
				left = (w.max[i] - size) / 2
				right = w.max[i] - left

			default:
				left = 0
				right = w.max[i] - size
			}

			if i > 0 {
				fmt.Fprint(buf, " ")
			}
			fmt.Fprint(buf, strings.Repeat(" ", left))
			fmt.Fprint(buf, row[i])
			fmt.Fprint(buf, strings.Repeat(" ", right))
		}

		fmt.Fprintln(output, buf.String())
	}
	w.rows = w.rows[:0]
}
