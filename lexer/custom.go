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

package lexer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ianlewis/runeio"
)

// EOF is a rune that indicates that the lexer has finished processing.
var EOF rune = -1

// LexState is the state of the current lexing state machine. It defines the logic
// to process the current state and returns the next state.
type LexState interface {
	// Run returns the next state to transition to or an error. If the returned
	// next state is nil or the returned error is io.EOF then the Lexer
	// finishes processing normally.
	Run(context.Context, *CustomLexer) (LexState, error)
}

type lexFnState struct {
	f func(context.Context, *CustomLexer) (LexState, error)
}

// Run implements LexState.Run.
func (s *lexFnState) Run(ctx context.Context, l *CustomLexer) (LexState, error) {
	if s.f == nil {
		return nil, nil
	}
	return s.f(ctx, l)
}

// LexStateFn creates a State from the given Run function.
func LexStateFn(f func(context.Context, *CustomLexer) (LexState, error)) LexState {
	return &lexFnState{f}
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

// NewCustomLexer creates a new Lexer initialized with the given starting [LexState].
// The Lexer takes ownership of the tokens channel and closes it when lexing
// is completed.
func NewCustomLexer(r io.Reader, startingState LexState) *CustomLexer {
	l := &CustomLexer{
		state: startingState,
		pos: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
		cursor: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
	}

	// If already a *bufio.Reader, use it directly.
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}

	l.r = runeio.NewReader(br)

	return l
}

// Pos returns the current position of the underlying reader.
func (l *CustomLexer) Pos() Position {
	return l.pos
}

// Cursor returns the current position of the underlying cursor marking the
// beginning of the current token being processed.
func (l *CustomLexer) Cursor() Position {
	return l.cursor
}

// Token returns the current token value.
func (l *CustomLexer) Token() string {
	return l.b.String()
}

// Width returns the current width of the token being processed. It is
// equivalent to l.Pos().Offset - l.Cursor().Offset.
func (l *CustomLexer) Width() int {
	return l.pos.Offset - l.cursor.Offset
}

// NextRune returns the next rune of input, advancing the reader while not
// advancing the token cursor.
func (l *CustomLexer) NextRune() rune {
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

// NextToken implements [Lexer.NextToken] and returns the next token from the
// input stream. If the end of the input is reached, a token with type
// [TokenTypeEOF] is returned.
func (l *CustomLexer) NextToken(ctx context.Context) *Token {
	if l.err != nil {
		return l.newToken(TokenTypeEOF)
	}

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
		l.state, err = l.state.Run(ctx, l)
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
	l.err = io.EOF
	return l.newToken(TokenTypeEOF)
}

// Peek returns the next rune from the buffer without advancing the reader or
// current token cursor.
func (l *CustomLexer) Peek() rune {
	p := l.PeekN(1)
	if len(p) < 1 {
		return EOF
	}
	return p[0]
}

// PeekN returns the next n runes from the buffer without advancing the reader
// or current token cursor. PeekN may return fewer runes than requested if an
// error occurs or at end of input.
func (l *CustomLexer) PeekN(n int) []rune {
	if l.err != nil {
		return nil
	}

	p, err := l.r.Peek(n)
	l.setErr(err)
	return p
}

// Advance attempts to advance the underlying reader a single rune and returns
// true if actually advanced. The current token cursor position is not updated.
func (l *CustomLexer) Advance() bool {
	return l.advance(1, false) == 1
}

// AdvanceN attempts to advance the underlying reader n runes and returns the
// number actually advanced. The current token cursor position is not updated.
func (l *CustomLexer) AdvanceN(n int) int {
	return l.advance(n, false)
}

// Discard attempts to discard the next rune, advancing the current token
// cursor, and returns true if actually discarded.
func (l *CustomLexer) Discard() bool {
	return l.DiscardN(1) == 1
}

// DiscardN attempts to discard n runes, advancing the current token cursor
// position, and returns the number actually discarded.
func (l *CustomLexer) DiscardN(n int) int {
	return l.advance(n, true)
}

// advance attempts to advance the reader n runes. If discard is true the token
// cursor position is updated as well.
func (l *CustomLexer) advance(n int, discard bool) int {
	if l.err != nil {
		return 0
	}
	var advanced int
	if discard {
		defer l.Ignore()
	}
	// We will attempt to do a zero-copy read by peeking at no more than what is
	// currently buffered in the reader operating on a slice that points
	// directly to the buffer's memory.
	// Minimum size the buffer of underlying reader could be expected to be.
	minSize := 16
	for n > 0 {
		// Determine the number of runes to read.
		toRead := l.r.Buffered()
		if n < toRead {
			toRead = n
		}
		if toRead == 0 {
			if minSize < n {
				toRead = minSize
			} else {
				toRead = n
			}
		}
		// Peek at input so we can increment position, line, column counters.
		rn, err := l.r.Peek(toRead)
		if err != nil && !errors.Is(err, io.EOF) {
			l.setErr(fmt.Errorf("peeking input: %w", err))
			return advanced
		}
		// Advance by peeked amount.
		d, dErr := l.r.Discard(len(rn))
		advanced += d
		l.pos.Offset += d
		// NOTE: We must be careful since toRead could be different from # of
		// runes peeked and/or discarded. We will only actually advance by the
		// number of runes discarded in the underlying reader to maintain
		// consistency.
		for i := 0; i < d; i++ {
			if rn[i] == '\n' {
				l.pos.Line++
				l.pos.Column = 1
			} else {
				l.pos.Column++
			}
		}
		if !discard {
			l.b.WriteString(string(rn))
		}
		if dErr != nil {
			l.setErr(fmt.Errorf("discarding input: %w", err))
			return advanced
		}
		if err != nil {
			// EOF from Peek
			l.setErr(err)
			return advanced
		}
		n -= d
	}
	return advanced
}

// Find searches the input for one of the given search strings, advancing the
// reader, and stopping when one of the strings is found. The token cursor is
// not advanced. The string found is returned. If no match is found an empty
// string is returned.
func (l *CustomLexer) Find(q []string) string {
	var maxLen int
	for i := range q {
		if len(q[i]) > maxLen {
			maxLen = len(q[i])
		}
	}

	if maxLen == 0 {
		return ""
	}

	// TODO(#94): use backtracking
	for {
		// Continue until PeekN can't get any new runes or we find a string
		// we're looking for.
		rns := l.PeekN(maxLen)
		if len(rns) == 0 {
			return ""
		}

		for j := range q {
			if strings.HasPrefix(string(rns), q[j]) {
				return q[j]
			}
		}

		_ = l.NextRune()
	}
}

// DiscardTo searches the input for one of the given search strings, advancing the
// reader, and stopping when one of the strings is found. The token cursor is
// advanced and data prior to the search string is discarded. The string found is
// returned. If no match is found an empty string is returned.
func (l *CustomLexer) DiscardTo(q []string) string {
	var maxLen int
	for i := range q {
		if len(q[i]) > maxLen {
			maxLen = len(q[i])
		}
	}

	if maxLen == 0 {
		return ""
	}

	for {
		bufS := l.r.Buffered()
		if bufS < maxLen {
			bufS = maxLen
		}

		// TODO(#94): use backtracking
		rns := l.PeekN(bufS)
		for i := 0; i < len(rns)-maxLen+1; i++ {
			for j := range q {
				if strings.HasPrefix(string(rns[i:i+maxLen]), q[j]) {
					// We have found a match. Discard prior runes and return.
					if n := l.advance(i, true); n < i {
						// We should have been able to advance by this amount.
						// An error has likely occurred.
						return ""
					}
					return q[j]
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

// Ignore ignores the previous input and resets the token start position to
// the current reader position.
func (l *CustomLexer) Ignore() {
	l.cursor = l.pos
	l.b.Reset()
}

// Emit emits the token between the current cursor position and reader
// position and returns the token. If the lexer is not currently active, this
// is a no-op. This advances the current token cursor.
func (l *CustomLexer) Emit(typ TokenType) *Token {
	if l.err != nil {
		return nil
	}

	token := l.newToken(typ)

	l.buf = append(l.buf, token)
	l.Ignore()

	return token
}

func (l *CustomLexer) newToken(typ TokenType) *Token {
	return &Token{
		Type:  typ,
		Value: l.b.String(),
		Start: l.cursor,
		End:   l.pos,
	}
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
