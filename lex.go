// A generic helper library for writing your own lexers, based on Rob Pike's
// presentation at https://www.youtube.com/watch?v=HxaD_trXwRE
//
// It will be helpful to look at the (well documented) example included in this
// repo in order to really understand how to use this package
package lexgo

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"unicode"
)

var (
	errInvalidUTF8 = errors.New("invalid utf8 character")
)

// Enumerator type for different types of tokens. You have to define the actual
// enumerations yourself
type TokenType int

const (
	// Represents an error encountered reading the byte stream (such as a
	// network error). This includes io.EOF.
	Err TokenType = iota

	// User defined Token types should start at this enumerator and increment
	// up. This is never actually returned by this library
	UserDefined
)

// Token represents a single set of characters of the given type. It also
// includes the row/column the characters started on
type Token struct {
	TokenType
	Val      string
	Row, Col int

	// If TokenType == Err this will contain the error being sent back.
	// Otherwise it will always be nil
	Err error
}

// Returns a nice string representation of the token
func (t *Token) String() string {
	var s string
	if t.Err != nil {
		s = t.Err.Error()
	} else {
		s = t.Val
	}
	return fmt.Sprintf(`{%d:%d,%d,%q}`, t.Row, t.Col, t.TokenType, s)
}

// A LexerFunc takes in an existing Lexer, uses it to read in a single rune,
// possibly Emit()'s a Token, and returns the next LexerFunc which should be
// executed
type LexerFunc func(*Lexer) LexerFunc

type Lexer struct {
	r      *bufio.Reader
	outbuf *bytes.Buffer
	ch     chan *Token
	state  LexerFunc

	// row/col the current token being buffered started out. Will be -1 if it
	// hasn't started yet
	row, col int

	// row/col of the rune most recently read. These are never reset (except
	// col, when a newline is reached)
	absRow, absCol int
}

// NewLexer constructs a new Lexer struct and returns it. r is internally
// wrapped with a bufio.Reader, unless it already is one. firstFunc is the
// LexerFunc which should be run on the first invocation of Next()
func NewLexer(r io.Reader, firstFunc LexerFunc) *Lexer {
	var br *bufio.Reader
	var ok bool
	if br, ok = r.(*bufio.Reader); !ok {
		br = bufio.NewReader(r)
	}

	l := Lexer{
		r:      br,
		ch:     make(chan *Token, 1),
		outbuf: bytes.NewBuffer(make([]byte, 0, 1024)),
		state:  firstFunc,
		row:    -1,
		col:    -1,
		absRow: 1,
	}

	return &l
}

// Returns the next Token Emit()'d
func (l *Lexer) Next() *Token {
	for {
		select {
		case t := <-l.ch:
			return t
		default:
			if l.state == nil {
				l.EmitErr(io.EOF)
			}
			l.state = l.state(l)
		}
	}
}

// Declares that the data buffered thusfar constitutes a Token. This will emit
// that Token to the next call of Next() and reset the buffer
func (l *Lexer) Emit(t TokenType) {
	str := l.outbuf.String()
	l.ch <- &Token{
		TokenType: t,
		Val:       str,
		Row:       l.row,
		Col:       l.col,
	}
	l.outbuf.Reset()
	l.row, l.col = -1, -1
}

// Used to Emit() and error which has occured. This will not affect the output
// buffer. It is not necessary to call on errors returned from ReadRune() or
// PeekRune()
func (l *Lexer) EmitErr(err error) {
	l.ch <- &Token{
		TokenType: Err,
		Err:       err,
	}
}

// Returns the next rune in the byte stream. If an error is returned it will
// have already been Emit()'d as an Err Token, but further handling can be done
// if necessary
func (l *Lexer) ReadRune() (rune, error) {
	r, err := l.readRune()
	if err != nil {
		return 0, err
	}

	if r == '\n' {
		l.absRow++
		l.absCol = 0
	} else {
		l.absCol++
	}

	return r, nil
}

func (l *Lexer) readRune() (rune, error) {
	r, i, err := l.r.ReadRune()
	if err != nil {
		l.EmitErr(err)
		return 0, err
	} else if r == unicode.ReplacementChar && i == 1 {
		l.EmitErr(errInvalidUTF8)
		return 0, errInvalidUTF8
	}

	return r, nil
}

// Returns the next rune which will appear in the byte stream without advancing
// the reader. In other words, multiple sequential calls to Peek() will return
// the same rune over and over, instead of returning sequential runes in the
// stream. Follows the same error semantics as ReadRune()
func (l *Lexer) PeekRune() (rune, error) {
	r, err := l.readRune()
	if err != nil {
		// No need to emitErr here, ReadRune already did it
		return 0, err
	}
	if err = l.r.UnreadRune(); err != nil {
		l.EmitErr(err)
		return 0, err
	}
	return r, nil
}

// Appends the given rune to the output buffer. When a full Token has been
// collected in this buffer Emit() can be used to emit that Token and clear the
// buffer at the same time
func (l *Lexer) BufferRune(r rune) {
	l.outbuf.WriteRune(r)

	if l.row < 0 && l.col < 0 {
		l.row, l.col = l.absRow, l.absCol
	}
}
