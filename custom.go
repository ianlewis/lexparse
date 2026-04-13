// Copyright 2023 Google LLC
// Copyright 2025 Ian Lewis
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lexparse

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ianlewis/runeio"
)

// EOF is a rune that indicates that the lexer has finished processing.
const EOF rune = -1

// LexState is the state of the current lexing state machine. It defines the logic
// to process the current state and returns the next state.
type LexState interface {
	// Run returns the next state to transition to or an error. If the returned
	// error is io.EOF then the Lexer finishes processing normally.
	Run(ctx context.Context, cur *LexCursor) (LexState, error)
}

type lexFnState struct {
	f func(context.Context, *LexCursor) (LexState, error)
}

// Run implements [LexState.Run].
//
//nolint:ireturn // Returning interface required to satisfy [LexState.Run]
func (s *lexFnState) Run(ctx context.Context, cur *LexCursor) (LexState, error) {
	return s.f(ctx, cur)
}

// LexStateFn creates a State from the given Run function.
//
//nolint:ireturn // Returning interface required to satisfy [LexState.Run]
func LexStateFn(f func(context.Context, *LexCursor) (LexState, error)) LexState {
	return &lexFnState{f}
}

// LexCursor is a type that allows for processing the input for a
// [CustomLexer]. It provides methods to advance the reader, emit tokens, and
// manage the current token being processed. It is designed to be used within
// the [LexState.Run] method to allow the state implementation to interact with
// the lexer without exposing the full CustomLexer implementation.
type LexCursor struct {
	l *CustomLexer
}

// NewLexCursor creates a new [LexCursor].
func NewLexCursor(l *CustomLexer) *LexCursor {
	return &LexCursor{
		l: l,
	}
}

// Advance attempts to advance the underlying reader a single rune and returns
// true if actually advanced. The current token cursor position is not updated.
func (c *LexCursor) Advance() bool {
	return c.l.advance(1, false) == 1
}

// AdvanceN attempts to advance the underlying reader n runes and returns the
// number actually advanced. The current token cursor position is not updated.
func (c *LexCursor) AdvanceN(n int) int {
	return c.l.advance(n, false)
}

// Cursor returns the current position of the underlying cursor marking the
// beginning of the current token being processed.
func (c *LexCursor) Cursor() Position {
	return c.l.cursor
}

// Discard attempts to discard the next rune, advancing the current token
// cursor, and returns true if actually discarded.
func (c *LexCursor) Discard() bool {
	return c.l.advance(1, true) == 1
}

// DiscardN attempts to discard n runes, advancing the current token cursor
// position, and returns the number actually discarded.
func (c *LexCursor) DiscardN(n int) int {
	return c.l.advance(n, true)
}

// DiscardTo searches the input for one of the given search strings, advancing
// the reader, and stopping when one of the strings is found. The token cursor
// is advanced and data prior to the search string is discarded. The string
// found is returned. If no match is found an empty string is returned.
func (c *LexCursor) DiscardTo(query []string) string {
	return c.l.discardTo(query)
}

// Emit emits the token between the current cursor position and reader
// position and returns the token. If the lexer is not currently active, this
// is a no-op. This advances the current token cursor.
func (c *LexCursor) Emit(typ TokenType) *Token {
	return c.l.emit(typ)
}

// Find searches the input for one of the given search strings, advancing the
// reader, and stopping when one of the strings is found. The token cursor is
// not advanced. The string found is returned. If no match is found an empty
// string is returned.
func (c *LexCursor) Find(query []string) string {
	return c.l.find(query)
}

// Ignore ignores the previous input and resets the token start position to
// the current reader position.
func (c *LexCursor) Ignore() {
	c.l.ignore()
}

// NextRune returns the next rune of input, advancing the reader while not
// advancing the token cursor.
func (c *LexCursor) NextRune() rune {
	return c.l.nextRune()
}

// Peek returns the next rune from the buffer without advancing the reader or
// current token cursor.
func (c *LexCursor) Peek() rune {
	p := c.PeekN(1)
	if len(p) < 1 {
		return EOF
	}

	return p[0]
}

// PeekN returns the next n runes from the buffer without advancing the reader
// or current token cursor. PeekN may return fewer runes than requested if an
// error occurs or at end of input.
func (c *LexCursor) PeekN(n int) []rune {
	return c.l.peekN(n)
}

// Pos returns the current position of the underlying reader.
func (c *LexCursor) Pos() Position {
	return c.l.pos
}

// Token returns the current token value.
func (c *LexCursor) Token() string {
	return c.l.b.String()
}

// Width returns the current width of the token being processed. It is
// equivalent to l.Pos().Offset - l.Cursor().Offset.
func (c *LexCursor) Width() int {
	return c.l.pos.Offset - c.l.cursor.Offset
}

// CustomLexer lexically processes a byte stream. It is implemented as a
// finite-state machine in which each [LexState] implements it's own processing.
//
// A CustomLexer maintains an internal cursor which marks the start of the next
// token being currently processed. The Lexer can then advance the reader to
// find the end of the token before emitting it.
type CustomLexer struct {
	// buf is a buffer of tokens that have been emitted but not yet processed.
	buf []*Token

	// state is the current state of the Lexer.
	state LexState

	// r is the underlying reader to read from.
	r *runeio.RuneReader

	// b is a strings builder that stores the current token value.
	b strings.Builder

	// pos is the current position of the underlying reader.
	pos Position

	// cursor is the start position of the current token.
	cursor Position

	// err is the first error the lexer encountered.
	err error
}

// NewCustomLexer creates a new Lexer initialized with the given starting
// [LexState]. The Lexer takes ownership of the tokens channel and closes it
// when lexing is completed.
func NewCustomLexer(reader io.Reader, startingState LexState) *CustomLexer {
	var fileName string

	file, isFile := reader.(*os.File)
	if isFile {
		fileName = file.Name()
	}

	customLexer := &CustomLexer{
		state: startingState,
		pos: Position{
			Filename: fileName,
			Offset:   0,
			Line:     1,
			Column:   1,
		},
		cursor: Position{
			Filename: fileName,
			Offset:   0,
			Line:     1,
			Column:   1,
		},
	}

	// If already a *bufio.Reader, use it directly.
	br, isBufReader := reader.(*bufio.Reader)
	if !isBufReader {
		br = bufio.NewReader(reader)
	}

	customLexer.r = runeio.NewReader(br)

	return customLexer
}

// NextToken implements [Lexer.NextToken] and returns the next token from the
// input stream. If the end of the input is reached, a token with type
// [TokenTypeEOF] is returned.
func (l *CustomLexer) NextToken(ctx context.Context) *Token {
	if l.err != nil {
		return l.newToken(TokenTypeEOF)
	}

	cur := NewLexCursor(l)

	// If we have no tokens to return, we need to run the current state.
	for len(l.buf) == 0 && l.state != nil {
		// Return EOF if the context is done/canceled. Don't rely on l.state.Run
		// implementation to check the context.
		select {
		case <-ctx.Done():
			l.setErr(ctx.Err())
			return l.newToken(TokenTypeEOF)
		default:
		}

		var err error

		l.state, err = l.state.Run(ctx, cur)
		l.setErr(err)

		if l.err != nil {
			return l.newToken(TokenTypeEOF)
		}
	}

	if len(l.buf) > 0 {
		// If we have already emitted tokens, return the next one.
		token := l.buf[0]
		if token.Type != TokenTypeEOF {
			l.buf = l.buf[1:]
		}

		return token
	}

	// The state is nil and we have no tokens to return, so we are at the end
	// of the input.
	return l.newToken(TokenTypeEOF)
}

func (l *CustomLexer) nextRune() rune {
	if l.err != nil {
		return EOF
	}

	rn, _, err := l.r.ReadRune()
	if err != nil {
		l.setErr(err)
		return EOF
	}

	l.pos.Offset++

	l.pos.Column++
	if rn == '\n' {
		l.pos.Line++
		l.pos.Column = 1
	}

	_, _ = l.b.WriteRune(rn)

	return rn
}

// advance attempts to advance the reader numRunes runes. If discard is true
// then the token cursor position is updated as well.
func (l *CustomLexer) advance(numRunes int, discard bool) int {
	if l.err != nil {
		return 0
	}

	var advanced int

	if discard {
		defer l.ignore()
	}

	// We will attempt to do a zero-copy read by peeking at no more than what is
	// currently buffered in the reader operating on a slice that points
	// directly to the buffer's memory.
	// Minimum size the buffer of underlying reader could be expected to be.
	minSize := 16

	for numRunes > 0 {
		// Determine the number of runes to read.
		toRead := min(l.r.Buffered(), numRunes)

		if toRead == 0 {
			// Nothing is currently buffered. Read at most minSize or numRunes,
			// whichever is smaller.
			toRead = min(numRunes, minSize)
		}

		// Peek at the input so we can increment the position, line, and column
		// counters.
		peekedRunes, peekErr := l.r.Peek(toRead)
		if peekErr != nil && !errors.Is(peekErr, io.EOF) {
			l.setErr(fmt.Errorf("peeking input: %w", peekErr))
			return advanced
		}

		// Advance by peeked amount.
		numDiscarded, dErr := l.r.Discard(len(peekedRunes))
		advanced += numDiscarded
		l.pos.Offset += numDiscarded

		// NOTE: We must be careful since toRead could be different from # of
		// runes peeked and/or discarded. We will only actually advance by the
		// number of runes discarded in the underlying reader to maintain
		// consistency.
		for i := range numDiscarded {
			if peekedRunes[i] == '\n' {
				l.pos.Line++
				l.pos.Column = 1
			} else {
				l.pos.Column++
			}
		}

		if !discard {
			l.b.WriteString(string(peekedRunes))
		}

		if dErr != nil {
			l.setErr(fmt.Errorf("discarding input: %w", dErr))
			return advanced
		}

		if peekErr != nil {
			// EOF from Peek
			return advanced
		}

		numRunes -= numDiscarded
	}

	return advanced
}

func (l *CustomLexer) discardTo(query []string) string {
	var maxLen int
	for i := range query {
		if len(query[i]) > maxLen {
			maxLen = len(query[i])
		}
	}

	if maxLen == 0 {
		return ""
	}

	for {
		bufS := max(l.r.Buffered(), maxLen)

		// TODO(#94): use backtracking
		rns := l.peekN(bufS)
		for i := range len(rns) - maxLen + 1 {
			for j := range query {
				if strings.HasPrefix(string(rns[i:i+maxLen]), query[j]) {
					// We have found a match. Discard prior runes and return.
					if n := l.advance(i, true); n < i {
						// We should have been able to advance by this amount.
						// An error has likely occurred.
						return ""
					}

					return query[j]
				}
			}
		}

		// Advance the reader by the runes peeked checked.
		// NOTE: Only advance the reader the number of runes that could never
		// match the substring. Not the full number peeked.
		toDiscard := len(rns) - maxLen + 1
		if toDiscard <= 0 {
			toDiscard = 1
		}

		if n := l.advance(toDiscard, true); n < toDiscard {
			// We should have been able to advance by this amount.
			// An error has likely occurred.
			return ""
		}
	}
}

func (l *CustomLexer) emit(typ TokenType) *Token {
	if l.err != nil {
		return nil
	}

	token := l.newToken(typ)

	l.buf = append(l.buf, token)
	l.ignore()

	return token
}

func (l *CustomLexer) find(query []string) string {
	var maxLen int
	for i := range query {
		if len(query[i]) > maxLen {
			maxLen = len(query[i])
		}
	}

	if maxLen == 0 {
		return ""
	}

	// TODO(#94): use backtracking
	for {
		// Continue until PeekN can't get any new runes or we find a string
		// we're looking for.
		rns := l.peekN(maxLen)
		if len(rns) == 0 {
			return ""
		}

		for j := range query {
			if strings.HasPrefix(string(rns), query[j]) {
				return query[j]
			}
		}

		_ = l.nextRune()
	}
}

func (l *CustomLexer) ignore() {
	l.cursor = l.pos
	l.b.Reset()
}

// newToken creates a new token starting from the current cursor position to the
// current reader position.
func (l *CustomLexer) newToken(typ TokenType) *Token {
	return &Token{
		Type:  typ,
		Value: l.b.String(),
		Start: l.cursor,
		End:   l.pos,
	}
}

func (l *CustomLexer) peekN(n int) []rune {
	if l.err != nil {
		return nil
	}

	p, err := l.r.Peek(n)
	l.setErr(err)

	return p
}

// Err returns any errors that the lexer encountered.
func (l *CustomLexer) Err() error {
	return l.err
}

func (l *CustomLexer) setErr(err error) {
	if l.err == nil && !errors.Is(err, io.EOF) {
		l.err = err
	}
}

// SetFilename sets the filename in the lexer's positional information.
func (l *CustomLexer) SetFilename(name string) {
	l.pos.Filename = name
	l.cursor.Filename = name
}
