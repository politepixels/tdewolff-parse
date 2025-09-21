package parse

import (
	"bytes"
	"io"
	"unicode/utf8"
)

var nullBuffer = []byte{0}

// Input is a buffered reader that allows peeking forward and shifting, taking an io.Input.
// It keeps data in-memory until Free, taking a byte length, is called to move beyond the data.
type Input struct {
	buf   []byte
	pos   int // index in buf
	start int // index in buf
	err   error

	restore func()

	line        int // current line number (1-based)
	col         int // current column number (1-based, in runes)
	lastNewline int // byte offset of the last newline character
}

// NewInput returns a new Input for a given io.Input and uses io.ReadAll to read it into a byte slice.
// If the io.Input implements Bytes, that is used instead. It will append a NULL at the end of the buffer.
func NewInput(r io.Reader) *Input {
	var b []byte
	if r != nil {
		if buffer, ok := r.(interface {
			Bytes() []byte
		}); ok {
			b = buffer.Bytes()
		} else {
			var err error
			b, err = io.ReadAll(r)
			if err != nil {
				return &Input{
					buf:  nullBuffer,
					err:  err,
					line: 1,
					col:  1,
				}
			}
		}
	}
	return NewInputBytes(b)
}

// NewInputString returns a new Input for a given string and appends NULL at the end.
func NewInputString(s string) *Input {
	return NewInputBytes([]byte(s))
}

// NewInputBytes returns a new Input for a given byte slice and appends NULL at the end.
// To avoid reallocation, make sure the capacity has room for one more byte.
func NewInputBytes(b []byte) *Input {
	z := &Input{
		buf:         b,
		line:        1,
		col:         1,
		lastNewline: -1,
	}

	n := len(b)
	if n == 0 {
		z.buf = nullBuffer
	} else {
		// Append NULL to buffer, but try to avoid reallocation
		if cap(b) > n {
			// Overwrite next byte but restore when done
			b = b[:n+1]
			c := b[n]
			b[n] = 0

			z.buf = b
			z.restore = func() {
				b[n] = c
			}
		} else {
			z.buf = append(b, 0)
		}
	}
	return z
}

// Restore restores the replaced byte past the end of the buffer by NULL.
func (z *Input) Restore() {
	if z.restore != nil {
		z.restore()
		z.restore = nil
	}
}

// Err returns the error returned from io.Input or io.EOF when the end has been reached.
func (z *Input) Err() error {
	return z.PeekErr(0)
}

// PeekErr returns the error at position pos. When pos is zero, this is the same as calling Err().
func (z *Input) PeekErr(pos int) error {
	if z.err != nil {
		return z.err
	} else if len(z.buf)-1 <= z.pos+pos {
		return io.EOF
	}
	return nil
}

// Peek returns the ith byte relative to the end position.
// Peek returns 0 when an error has occurred, Err returns the erroz.
func (z *Input) Peek(pos int) byte {
	pos += z.pos
	return z.buf[pos]
}

// PeekRune returns the rune and rune length of the ith byte relative to the end position.
func (z *Input) PeekRune(pos int) (rune, int) {
	// from unicode/utf8
	c := z.Peek(pos)
	if c < 0xC0 || len(z.buf)-1-z.pos < 2 {
		return rune(c), 1
	} else if c < 0xE0 || len(z.buf)-1-z.pos < 3 {
		return rune(c&0x1F)<<6 | rune(z.Peek(pos+1)&0x3F), 2
	} else if c < 0xF0 || len(z.buf)-1-z.pos < 4 {
		return rune(c&0x0F)<<12 | rune(z.Peek(pos+1)&0x3F)<<6 | rune(z.Peek(pos+2)&0x3F), 3
	}
	return rune(c&0x07)<<18 | rune(z.Peek(pos+1)&0x3F)<<12 | rune(z.Peek(pos+2)&0x3F)<<6 | rune(z.Peek(pos+3)&0x3F), 4
}

// Move advances the position and updates the line and column counters.
func (z *Input) Move(n int) {
	if n <= 0 {
		return
	}
	end := z.pos + n
	if end > len(z.buf)-1 {
		end = len(z.buf) - 1
	}

	// Scan only the moved segment for newlines and runes.
	movedBytes := z.buf[z.pos:end]
	newlines := bytes.Count(movedBytes, []byte{'\n'})
	if newlines > 0 {
		z.line += newlines
		z.lastNewline = z.pos + bytes.LastIndexByte(movedBytes, '\n')
		z.col = utf8.RuneCount(z.buf[z.lastNewline+1:end]) + 1
	} else {
		z.col += utf8.RuneCount(movedBytes)
	}
	z.pos = end
}

// MoveRune advances the position by the length of the current rune.
func (z *Input) MoveRune() {
	_, n := z.PeekRune(0)
	z.Move(n)
}

// Pos returns a mark to which can be rewinded.
func (z *Input) Pos() int {
	return z.pos - z.start
}

// Rewind rewinds the position to the given position.
func (z *Input) Rewind(pos int) {
	z.pos = z.start + pos
}

// Lexeme returns the bytes of the current selection.
func (z *Input) Lexeme() []byte {
	return z.buf[z.start:z.pos:z.pos]
}

// Skip collapses the position to the end of the selection.
func (z *Input) Skip() {
	z.start = z.pos
}

// Shift returns the bytes of the current selection and collapses the position to the end of the selection.
func (z *Input) Shift() []byte {
	b := z.buf[z.start:z.pos:z.pos]
	z.start = z.pos
	return b
}

// Offset returns the character position in the buffez.
func (z *Input) Offset() int {
	return z.pos
}

// Bytes returns the underlying buffez.
func (z *Input) Bytes() []byte {
	return z.buf[: len(z.buf)-1 : len(z.buf)-1]
}

// Len returns the length of the underlying buffez.
func (z *Input) Len() int {
	return len(z.buf) - 1
}

// Reset resets position to the underlying buffez.
func (z *Input) Reset() {
	z.start = 0
	z.pos = 0
	z.line = 1
	z.col = 1
	z.lastNewline = -1
}

// Position returns the current line and column number.
func (z *Input) Position() (line, col int) {
	return z.line, z.col
}

// PositionAt returns the line and column number for an arbitrary offset.
func (z *Input) PositionAt(offset int) (line, col int) {
	if offset > len(z.buf) {
		offset = len(z.buf)
	}

	lastNewline := bytes.LastIndexByte(z.buf[:offset], '\n')

	line = bytes.Count(z.buf[:lastNewline+1], []byte{'\n'}) + 1

	col = utf8.RuneCount(z.buf[lastNewline+1:offset]) + 1
	return line, col
}
