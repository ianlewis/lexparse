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
	"sync"
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

// LexemeType is a user-defined Lexeme type.
type LexemeType int

// State is the state of the current lexing state machine. It defines the logic
// to process the current state and returns the next state.
type State interface {
	// Run returns the next state to transition to or an error. If the returned
	// next state is nil or the returned error is io.EOF then the Lexer
	// finishes processing normally.
	Run(context.Context, *Lexer) (State, error)
}

type fnState struct {
	f func(context.Context, *Lexer) (State, error)
}

func (s *fnState) Run(ctx context.Context, l *Lexer) (State, error) {
	if s.f == nil {
		return nil, nil
	}
	return s.f(ctx, l)
}

// StateFn creates a State from the given Run function.
func StateFn(f func(context.Context, *Lexer) (State, error)) State {
	return &fnState{f}
}

// Lexeme is a tokenized input which can be emitted by a Lexer.
type Lexeme struct {
	// Type is the Lexeme's type.
	Type LexemeType

	// Value is the Lexeme's value.
	Value string

	// Pos is the position in the byte stream where the Lexeme was found.
	Pos int

	// Line is the line numbero where the Lexeme was found.
	Line int

	// Column is the column in the line where the Lexeme was found.
	Column int
}

// Lexer lexically processes a byte stream. It is implemented as a finite-state
// machine in which each State implements it's own processing.
type Lexer struct {
	// lexemes is a channel into which Lexeme's will be emitted.
	lexemes chan *Lexeme

	// done is the stop channel
	done chan struct{}

	// state is the current state of the Lexer.
	state State

	// s is the current input/pos/lexeme state.
	s struct {
		// Mutex protects the values in s.
		sync.Mutex

		// r is the underlying reader to read from.
		r BufferedRuneReader

		// b is a strings builder that stores the current lexeme value.
		b strings.Builder

		// pos is the current position in the input stream.
		pos int

		// line is the current line in the input.
		line int

		// column is the current column in the input.
		column int

		// startPos is the position of the current lexeme.
		startPos int

		// startLine is the line of the current lexeme.
		startLine int

		// startColumn is the column of the current lexeme.
		startColumn int

		// err holds the last lexing error.
		err error
	}
}

// NewLexer creates a new Lexer initialized with the given starting state.
func NewLexer(r BufferedRuneReader, startingState State) *Lexer {
	l := &Lexer{
		state:   startingState,
		lexemes: make(chan *Lexeme),
		done:    make(chan struct{}),
	}
	l.s.r = r
	return l
}

// Pos returns the current position of the underlying reader.
func (l *Lexer) Pos() int {
	l.s.Lock()
	pos := l.s.pos
	l.s.Unlock()
	return pos
}

// Line returns the current line in the input (zero indexed).
func (l *Lexer) Line() int {
	l.s.Lock()
	line := l.s.line
	l.s.Unlock()
	return line
}

// Column returns the current column in the input (zero indexed).
func (l *Lexer) Column() int {
	l.s.Lock()
	c := l.s.column
	l.s.Unlock()
	return c
}

// ReadRune returns the next rune of input.
func (l *Lexer) ReadRune() (rune, int, error) {
	l.s.Lock()
	rn, i, err := l.readrune()
	l.s.Unlock()
	return rn, i, err
}

func (l *Lexer) readrune() (rune, int, error) {
	rn, n, err := l.s.r.ReadRune()
	if err != nil {
		//nolint:wrapcheck // Error doesn't need to be wrapped.
		return 0, 0, err
	}

	l.s.pos++
	l.s.column++
	if rn == '\n' {
		l.s.line++
		l.s.column = 0
	}

	_, _ = l.s.b.WriteRune(rn)
	return rn, n, nil
}

// Peek returns the next n runes from the buffer without advancing the
// lexer or underlying reader. The runes stop being valid at the next read
// call. If Peek returns fewer than n runes, it also returns an error
// indicating why the read is short.
func (l *Lexer) Peek(n int) ([]rune, error) {
	l.s.Lock()
	p, err := l.s.r.Peek(n)
	l.s.Unlock()
	//nolint:wrapcheck // Error doesn't need to be wrapped.
	return p, err
}

// Advance attempts to advance the underlying reader n runes and returns the
// number actually advanced. If the number of runes advanced is different than
// n, then an error is returned explaining the reason. It also updates the
// current lexeme position.
func (l *Lexer) Advance(n int) (int, error) {
	l.s.Lock()
	a, err := l.advance(n, false)
	l.s.Unlock()
	return a, err
}

func (l *Lexer) advance(n int, discard bool) (int, error) {
	var advanced int
	if discard {
		defer l.ignore()
	}

	// Minimum size the buffer of underlying reader could be expected to be.
	minSize := 16
	for n > 0 {
		// Determine the number of runes to read.
		toRead := l.s.r.Buffered()
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
		rn, err := l.s.r.Peek(toRead)
		if err != nil && !errors.Is(err, io.EOF) {
			return advanced, fmt.Errorf("peeking input: %w", err)
		}

		// Advance by peeked amount.
		d, dErr := l.s.r.Discard(len(rn))
		advanced += d
		l.s.pos += d

		// NOTE: We must be careful since toRead could be different from #
		//       of runes peeked.
		for i := 0; i < d; i++ {
			if rn[i] == '\n' {
				l.s.line++
				l.s.column = 0
			} else {
				l.s.column++
			}
		}

		if !discard {
			l.s.b.WriteString(string(rn))
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

// Discard attempts to discard n runes and returns the number actually
// discarded. If the number of runes discarded is different than n, then an
// error is returned explaining the reason. It also resets the current lexeme
// position.
func (l *Lexer) Discard(n int) (int, error) {
	l.s.Lock()
	d, err := l.advance(n, true)
	l.s.Unlock()
	return d, err
}

// Find searches the input for one of the given tokens, advancing the reader,
// and stopping when one of the tokens is found. The token found is returned.
func (l *Lexer) Find(tokens []string) (string, error) {
	l.s.Lock()
	defer l.s.Unlock()

	var maxLen int
	for i := range tokens {
		if len(tokens[i]) > maxLen {
			maxLen = len(tokens[i])
		}
	}

	for {
		rns, err := l.s.r.Peek(maxLen)
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
// and stopping when one of the tokens is found. The data prior to the token is
// discarded. The token found is returned.
func (l *Lexer) SkipTo(tokens []string) (string, error) {
	l.s.Lock()
	defer l.s.Unlock()

	var maxLen int
	for i := range tokens {
		if len(tokens[i]) > maxLen {
			maxLen = len(tokens[i])
		}
	}

	for {
		bufS := l.s.r.Buffered()
		if bufS < maxLen {
			bufS = maxLen
		}

		rns, err := l.s.r.Peek(bufS)
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

// Ignore ignores the previous input and resets the lexeme start position to
// the current reader position.
func (l *Lexer) Ignore() {
	l.s.Lock()
	l.ignore()
	l.s.Unlock()
}

func (l *Lexer) ignore() {
	l.s.startPos = l.s.pos
	l.s.startLine = l.s.line
	l.s.startColumn = l.s.column
	l.s.b = strings.Builder{}
}

// Lex starts a new goroutine to parse the content. The caller can request that
// the lexer stop by cancelling ctx. The returned channel is closed when the
// Lexer is finished running.
func (l *Lexer) Lex(ctx context.Context) <-chan *Lexeme {
	// Just return if the lexer is already done.
	select {
	case <-l.Done():
		l.s.Unlock()
		return l.lexemes
	default:
	}

	go func() {
		var err error
		defer close(l.done)
		defer close(l.lexemes)
		for l.state != nil {
			select {
			case <-ctx.Done():
				l.setErr(ctx.Err())
				return
			default:
			}

			l.state, err = l.state.Run(ctx, l)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					l.setErr(err)
				}
				return
			}
		}
	}()
	return l.lexemes
}

// setErr sets the lexer's error value.
func (l *Lexer) setErr(err error) {
	l.s.Lock()
	l.s.err = err
	l.s.Unlock()
}

// Err returns the last encountered error.
func (l *Lexer) Err() error {
	l.s.Lock()
	err := l.s.err
	l.s.Unlock()
	return err
}

// Done returns a channel that is closed when the lexer is finished running.
func (l *Lexer) Done() <-chan struct{} {
	return l.done
}

// Lexeme returns a new Lexeme at the current position.
func (l *Lexer) Lexeme(typ LexemeType) *Lexeme {
	l.s.Lock()
	lexeme := &Lexeme{
		Type:   typ,
		Value:  l.s.b.String(),
		Pos:    l.s.startPos,
		Line:   l.s.startLine,
		Column: l.s.startColumn,
	}
	l.s.Unlock()
	return lexeme
}

// Emit is used by State implementations to emit a lexeme which will be passed
// on to the parser. If the lexer is not currently active, this is a no-op.
func (l *Lexer) Emit(lexeme *Lexeme) {
	if l.lexemes == nil {
		return
	}
	if lexeme == nil {
		return
	}
	l.lexemes <- lexeme
	l.Ignore()
}
