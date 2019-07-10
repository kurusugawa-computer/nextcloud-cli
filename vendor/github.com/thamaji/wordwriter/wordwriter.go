package wordwriter

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
	"golang.org/x/crypto/ssh/terminal"
)

func New(output io.Writer) *WordWriter {
	return &WordWriter{
		output: output,
	}
}

type WordWriter struct {
	output io.Writer
	words  []*word
}

type word struct {
	value string
	width int
}

var ansi = regexp.MustCompile("[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))")

func (w *WordWriter) Add(words ...string) {
	for _, value := range words {
		width := runewidth.StringWidth(ansi.ReplaceAllLiteralString(value, ""))
		w.words = append(w.words, &word{value: value, width: width})
	}
}

func (w *WordWriter) Flush() {
	if len(w.words) <= 0 {
		return
	}

	terminalWidth, _, err := terminal.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		terminalWidth = 0
	}

	cols := len(w.words)

	type column struct {
		width  int
		height int
	}

	var sample []column

	for ; cols > 0; cols-- {
		sample = make([]column, cols)

		rows := len(w.words) / cols
		for i := 0; i < len(sample); i++ {
			sample[i].height = rows
		}
		for i := 0; i < len(w.words)%cols; i++ {
			sample[i].height++
		}

		index := 0
		for col := 0; col < len(sample); col++ {
			for row := 0; row < sample[col].height; row++ {
				word := w.words[index]
				index++

				width := word.width
				if col != len(sample)-1 {
					width += 2 // これは列区切りのスペースのぶん！
				}

				if sample[col].width < width {
					sample[col].width = width
				}
			}
		}

		width := 0
		for i := range sample {
			width += sample[i].width
		}

		if width < terminalWidth {
			break
		}
	}

	buf := bytes.NewBuffer([]byte{})

	output := w.output
	if output == nil {
		output = os.Stdout
	}

	for row := 0; row < sample[0].height; row++ {
		buf.Reset()

		offset := 0
		for col := 0; col < len(sample); col++ {
			if row >= sample[col].height {
				break
			}

			word := w.words[offset+row]

			fmt.Fprint(buf, word.value)
			fmt.Fprint(buf, strings.Repeat(" ", sample[col].width-word.width))

			offset += sample[col].height
		}

		fmt.Fprintln(output, buf.String())
	}

	w.words = w.words[:0]
}
