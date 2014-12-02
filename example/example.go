package main

import (
	"fmt"
	"io"
	"os"
	"unicode"

	"github.com/mediocregopher/lexgo"
)

// Define your TokenTypes here. Make sure they start at UserDefined so they
// don't overlap with lexgo's builtin TokenTypes
const (
	OpenParen lexgo.TokenType = lexgo.UserDefined + iota
	CloseParen
	AlphaNum
)

// Wrap the lexgo Lexer so outside users of this package don't get confused
type LispLexer struct {
	lexer *lexgo.Lexer
}

func NewLispLexer(r io.Reader) *LispLexer {
	return &LispLexer{
		lexer: lexgo.NewLexer(r, lexWhitespace),
	}
}

// We expose Next(), but we don't want to expose anything else from Lexer since
// it's all only used internally
func (l *LispLexer) Next() *lexgo.Token {
	return l.lexer.Next()
}

// lexWhitespace is going to be the backbone of this particular lexer. It will
// read in runes, ignoring whitespace and deciding what to do with anything else
// it encounters
func lexWhitespace(lexer *lexgo.Lexer) lexgo.LexerFunc {
	r, err := lexer.ReadRune()
	if err != nil {
		// Errors from ReadRune() and PeekRune() should not be emitted, those
		// methods will do that for us. We return nil to indicate that the lexer
		// cannot progress. EOF will be returned for all future calls to Next()
		return nil
	}

	if unicode.IsSpace(r) {
		return lexWhitespace
	} else if r == '#' {
		return lexComment
	}

	// If r isn't whitespace or the start of a comment it's something we're
	// going to want to keep
	lexer.BufferRune(r)

	if r == '(' {
		lexer.Emit(OpenParen)
		return lexWhitespace
	} else if r == ')' {
		lexer.Emit(CloseParen)
		return lexWhitespace
	} else {
		return lexAlphaNum
	}
}

// lexComment will read, not buffering anything it reads so as to simply throw
// it away, until it sees a newline.
func lexComment(lexer *lexgo.Lexer) lexgo.LexerFunc {
	r, err := lexer.ReadRune()
	if err != nil {
		return nil
	}

	if r == '\n' {
		return lexWhitespace
	}
	return lexComment
}

// lexAlphaNum will buffer until it sees a non-alphanumeric character
func lexAlphaNum(lexer *lexgo.Lexer) lexgo.LexerFunc {
	r, err := lexer.PeekRune()
	if err != nil {
		return nil
	}

	if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
		// Whatever alpha-numeric string we were reading is over now, Emit() it
		// and go back to lexWhitespace
		lexer.Emit(AlphaNum)
		return lexWhitespace
	}

	// We call ReadRune() to consume the rune we already peeked. It's not
	// necessary to check the error on a rune you just peeked.
	lexer.ReadRune()
	lexer.BufferRune(r)
	return lexAlphaNum
}

// Putting our new awesome lexer to work!
func main() {

	f, err := os.Open("example.lisp")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	l := NewLispLexer(f)

	for {
		token := l.Next()
		if token.Err == io.EOF {
			fmt.Println("Done reading file!")
			return
		} else if token.Err != nil {
			panic(token.Err)
		}

		fmt.Println(token)
	}

}
