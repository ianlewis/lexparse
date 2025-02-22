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

// Token returns the current token token value.
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

// ReadRune returns the next rune of input, advancing the reader while not
// advancing the cursor.
func (l *Lexer) ReadRune() (rune, int, error) {
	return l.readrune()
}

func (l *Lexer) readrune() (rune, int, error) {
	rn, n, err := l.r.ReadRune()
	if err != nil {
		//nolint:wrapcheck // Error should not be wrapped as it could be io.EOF
		return 0, 0, err
	}

	l.pos++
	l.column++
	if rn == '\n' {
		l.line++
		l.column = 0
	}

	_, _ = l.b.WriteRune(rn)
	return rn, n, nil
}

// Peek returns the next n runes from the buffer without advancing the
// lexer or underlying reader. The runes stop being valid at the next read
// call. If Peek returns fewer than n runes, it also returns an error
// indicating why the read is short.
func (l *Lexer) Peek(n int) ([]rune, error) {
	p, err := l.r.Peek(n)
	//nolint:wrapcheck // Error may return io.EOF.
	return p, err
}

// Advance attempts to advance the underlying reader n runes and returns the
// number actually advanced. If the number of runes advanced is different than
// n, then an error is returned explaining the reason. It also updates the
// current token position.
func (l *Lexer) Advance(n int) (int, error) {
	return l.advance(n, false)
}

// Discard attempts to discard n runes and returns the number actually
// discarded. If the number of runes discarded is different than n, then an
// error is returned explaining the reason. It also advances the current token
// cursor position.
func (l *Lexer) Discard(n int) (int, error) {
	return l.advance(n, true)
}

func (l *Lexer) advance(n int, discard bool) (int, error) {
	var advanced int
	if discard {
		defer l.Ignore()
	}

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
			return advanced, fmt.Errorf("peeking input: %w", err)
		}

		// Advance by peeked amount.
		d, dErr := l.r.Discard(len(rn))
		advanced += d
		l.pos += d

		// NOTE: We must be careful since toRead could be different from #
		//       of runes peeked.
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
			return advanced, fmt.Errorf("discarding input: %w", err)
		}
		if err != nil {
			// EOF from Peek
			//nolint:wrapcheck // Error doesn't need to be wrapped.
			return advanced, err
		}

		n -= d
	}

	return advanced, nil
}

// Find searches the input for one of the given tokens, advancing the reader,
// and stopping when one of the tokens is found. The token cursor is not
// advanced. The token found is returned.
func (l *Lexer) Find(tokens []string) (string, error) {
	var maxLen int
	for i := range tokens {
		if len(tokens[i]) > maxLen {
			maxLen = len(tokens[i])
		}
	}

	for {
		rns, err := l.r.Peek(maxLen)
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("peeking input: %w", err)
		}
		for j := range tokens {
			if strings.HasPrefix(string(rns), tokens[j]) {
				return tokens[j], nil
			}
		}

		if _, _, err = l.readrune(); err != nil {
			return "", err
		}
	}
}

// SkipTo searches the input for one of the given tokens, advancing the reader,
// and stopping when one of the tokens is found. The token cursor is advanced
// and data prior to the token is discarded. The token found is returned.
func (l *Lexer) SkipTo(tokens []string) (string, error) {
	var maxLen int
	for i := range tokens {
		if len(tokens[i]) > maxLen {
			maxLen = len(tokens[i])
		}
	}

	for {
		bufS := l.r.Buffered()
		if bufS < maxLen {
			bufS = maxLen
		}

		rns, err := l.r.Peek(bufS)
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("peeking input: %w", err)
		}

		for i := 0; i < len(rns)-maxLen+1; i++ {
			for j := range tokens {
				if strings.HasPrefix(string(rns[i:i+maxLen]), tokens[j]) {
					// We have found a match. Discard prior runes and return.
					if _, advErr := l.advance(i, true); advErr != nil {
						return "", advErr
					}
					return tokens[j], nil
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
		if _, err = l.advance(toDiscard, true); err != nil {
			return "", err
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
	defer close(l.tokens)

	for l.state != nil {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("lexing: %w", err)
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
			return fmt.Errorf("lexing: %w", err)
		}
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("lexing: %w", err)
	}

	return nil
}

// Emit emits the token between the the current cursor position and reader
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
