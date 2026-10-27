package repl

import (
	"bufio"
	"os"
	"strings"

	"github.com/bugzi/gs/internal/complete"
	"golang.org/x/term"
)

// Reader 是 REPL 的行输入抽象
type Reader interface {
	Read() (string, error) // Ctrl-D 返回 io.EOF
	Close() error
	SetPrompt(p string)
}

// NewReader 默认自绘输入行；GS_PLAIN_INPUT=1 或 stdout 非 TTY 时降级 plain
func NewReader(prompt string, tree complete.Tree, hist *History) Reader {
	if os.Getenv("GS_PLAIN_INPUT") == "1" || !term.IsTerminal(int(os.Stdout.Fd())) {
		return newPlainReader(prompt)
	}
	return newRawReader(prompt, tree, hist)
}

// plainReader 降级实现：无历史无补全，但中文输入绝对正确
type plainReader struct {
	br     *bufio.Reader
	prompt string
}

func newPlainReader(prompt string) *plainReader {
	return &plainReader{br: bufio.NewReader(os.Stdin), prompt: prompt}
}

func (r *plainReader) Read() (string, error) {
	os.Stdout.WriteString(r.prompt)
	line, err := r.br.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (r *plainReader) SetPrompt(p string) { r.prompt = p }
func (r *plainReader) Close() error       { return nil }
