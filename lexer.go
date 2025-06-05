// Copyright 2023 Google LLC
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
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// BufferedRuneReader implements functionality that allows for allow for zero-copy
// reading of a rune stream.
type BufferedRuneReader interface {
	io.RuneReader

	// Buffered returns the number of runes currently buffered.
	//
	// This value becomes invalid following the next Read/Discard operation.
	Buffered() int

	// Peek returns the next n runes from the buffer without advancing the
	// reader. The runes stop being valid at the next read call. If Peek
	// returns fewer than n runes, it also returns an error indicating why the
	// read is short. ErrBufferFull is returned if n is larger than the
	// reader's buffer size.
	Peek(n int) ([]rune, error)

	// Discard attempts to discard n runes and returns the number actually
	// discarded. If the number of runes discarded is different than n, then an
	// error is returned explaining the reason.
	Discard(n int) (int, error)
}

// EOF is a rune that indicates that the lexer has finished processing.
var EOF rune = -1

// TokenType is a user-defined Token type.
type TokenType int

// LexState is the state of the current lexing state machine. It defines the logic
// to process the current state and returns the next state.
type LexState interface {
	// Run returns the next state to transition to or an error. If the returned
	// next state is nil or the returned error is io.EOF then the Lexer
	// finishes processing normally.
	Run(context.Context, *Lexer) (LexState, error)
}

type lexFnState struct {
	f func(context.Context, *Lexer) (LexState, error)
}

// Run implements LexState.Run.
func (s *lexFnState) Run(ctx context.Context, l *Lexer) (LexState, error) {
	if s.f == nil {
		return nil, nil
	}
	return s.f(ctx, l)
}

// LexStateFn creates a State from the given Run function.
func LexStateFn(f func(context.Context, *Lexer) (LexState, error)) LexState {
	return &lexFnState{f}
}

// Token is a tokenized input which can be emitted by a Lexer.
type Token struct {
	// Type is the Token's type.
	Type TokenType

	// Value is the Token's value.
	Value string

	// Pos is the position in the byte stream where the Token was found.
	Pos int

	// Line is the line number where the Token was found (one-based).
	Line int

	// Column is the column in the line where the Token was found (one-based).
	Column int
}

// TokenTypeEOF indicates an EOF token signalling the end of input.
var TokenTypeEOF TokenType = -1

// Lexer lexically processes a byte stream. It is implemented as a finite-state
// machine in which each [LexState] implements it's own processing.
//
// A Lexer maintains an internal cursor which marks the start of the next
// token being currently processed. The Lexer can then advance the reader to
// find the end of the token before emitting it.
type Lexer struct {
	// tokens is a channel into which [Token]'s will be emitted.
	tokens chan<- *Token

	// state is the current state of the Lexer.
	state LexState

	// r is the underlying reader to read from.
	r BufferedRuneReader

	// b is a strings builder that stores the current token value.
	b strings.Builder

	// pos is the current position in the input stream.
	pos int

	// line is the current line in the input.
	line int

	// column is the current column in the input.
	column int

	// startPos is the position of the current token.
	startPos int

	// startLine is the line of the current token.
	startLine int

	// startColumn is the column of the current token.
	startColumn int

	// err is the first error the lexer encountered.
	err error
}

// NewLexer creates a new Lexer initialized with the given starting [LexState].
// The Lexer takes ownership of the tokens channel and closes it when lexing
// is completed.
func NewLexer(r BufferedRuneReader, tokens chan<- *Token, startingState LexState) *Lexer {
	l := &Lexer{
		state:  startingState,
		tokens: tokens,
	}
	l.r = r
	return l
}

// Pos returns the current position of the underlying reader.
func (l *Lexer) Pos() int {
	return l.pos
}

// Cursor returns the current position of the underlying cursor marking the
// beginning of the current token being processed.
func (l *Lexer) Cursor() int {
	return l.startPos
}

// Token returns the current token value.
func (l *Lexer) Token() string {
	return l.b.String()
}

// Width returns the current width of the token being processed. It is
// equivalent to l.Pos() - l.Cursor().
func (l *Lexer) Width() int {
	return l.pos - l.startPos
}

// Line returns the current line in the input (one-based).
func (l *Lexer) Line() int {
	return l.line + 1
}

// Column returns the current column in the input (one-based).
func (l *Lexer) Column() int {
	return l.column + 1
}

// Next returns the next rune of input, advancing the reader while not
// advancing the cursor.
func (l *Lexer) Next() rune {
	if l.err != nil {
		return EOF
	}

	rn, _, err := l.r.ReadRune()
	if err != nil {
		l.setErr(err)
		return EOF
	}

	l.pos++
	l.column++
	if rn == '\n' {
		l.line++
		l.column = 0
	}

	_, _ = l.b.WriteRune(rn)
	return rn
}

// Peek returns the next rune from the buffer without advancing the reader or
// current token cursor.
func (l *Lexer) Peek() rune {
	p := l.PeekN(1)
	if len(p) < 1 {
		return EOF
	}
	return p[0]
}

// PeekN returns the next n runes from the buffer without advancing the reader
// or current token cursor. PeekN may return fewer runes than requested if an
// error occurs or at end of input.
func (l *Lexer) PeekN(n int) []rune {
	if l.err != nil {
		return nil
	}

	p, err := l.r.Peek(n)
	l.setErr(err)
	return p
}

// Advance attempts to advance the underlying reader a single rune and returns
// true if actually advanced. The current token cursor position is not updated.
func (l *Lexer) Advance() bool {
	return l.advance(1, false) == 1
}

// AdvanceN attempts to advance the underlying reader n runes and returns the
// number actually advanced. The current token cursor position is not updated.
func (l *Lexer) AdvanceN(n int) int {
	return l.advance(n, false)
}

// Discard attempts to discard the next rune, advancing the current token
// cursor, and returns true if actually discarded.
func (l *Lexer) Discard() bool {
	return l.DiscardN(1) == 1
}

// DiscardN attempts to discard n runes, advancing the current token cursor
// position, and returns the number actually discarded.
func (l *Lexer) DiscardN(n int) int {
	return l.advance(n, true)
}

// advance attempts to advance the reader n runes. If discard is true the token
// cursor position is updated as well.
func (l *Lexer) advance(n int, discard bool) int {
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
		l.pos += d

		// NOTE: We must be careful since toRead could be different from # of
		// runes peeked and/or discarded. We will only actually advance by the
		// number of runes discarded in the underlying reader to maintain
		// consistency.
		for i := 0; i < d; i++ {
			if rn[i] == '\n' {
				l.line++
				l.column = 0
			} else {
				l.column++
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
func (l *Lexer) Find(q []string) string {
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

		_ = l.Next()
	}
}

// DiscardTo searches the input for one of the given search strings, advancing the
// reader, and stopping when one of the strings is found. The token cursor is
// advanced and data prior to the search string is discarded. The string found is
// returned. If no match is found an empty string is returned.
func (l *Lexer) DiscardTo(q []string) string {
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
func (l *Lexer) Ignore() {
	l.startPos = l.pos
	l.startLine = l.line
	l.startColumn = l.column
	l.b.Reset()
}

// Lex parses the content and passes tokens to the given channel. Run is
// called on each [LexState] starting with the initial state. Each [LexState]
// then returns the subsequent state which is run until a state returns nil
// indicating that lexing has finished.
//
// Note that a separate goroutine should process tokens passed to the tokens
// channel or Lex will block indefinitely when the channel buffer is filled.
//
// The caller can request that the lexer stop by cancelling ctx.
func (l *Lexer) Lex(ctx context.Context) error {
	// Set the channel to support calls back into the Lexer.
	defer func() {
		l.Emit(TokenTypeEOF)
		close(l.tokens)
	}()

	for l.state != nil {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				//nolint:wrapcheck // Wrapping errors at this level is not user friendly.
				return err
			}
			return nil
		default:
		}

		var err error
		l.state, err = l.state.Run(ctx, l)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			//nolint:wrapcheck // Wrapping errors at this level is not user friendly.
			return err
		}

		// Return early if an error has occurred.
		if err := l.Err(); err != nil {
			return l.Err()
		}
	}

	if err := ctx.Err(); err != nil {
		//nolint:wrapcheck // Wrapping errors at this level is not user friendly.
		return err
	}

	return l.Err()
}

// Emit emits the token between the current cursor position and reader
// position and returns the token. If the lexer is not currently active, this
// is a no-op. This advances the current token cursor.
func (l *Lexer) Emit(typ TokenType) *Token {
	if l.tokens == nil {
		return nil
	}

	token := &Token{
		Type:   typ,
		Value:  l.b.String(),
		Pos:    l.startPos,
		Line:   l.startLine + 1,
		Column: l.startColumn + 1,
	}

	l.tokens <- token
	l.Ignore()

	return token
}

// Err returns any errors that the lexer encountered.
func (l *Lexer) Err() error {
	return l.err
}

func (l *Lexer) setErr(err error) {
	if l.err == nil && !errors.Is(err, io.EOF) {
		l.err = err
	}
}
