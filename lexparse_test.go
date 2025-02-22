// Copyright 2024 Google LLC
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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/ianlewis/runeio"
)

type parseWordState struct{}

func (w *parseWordState) Run(_ context.Context, p *Parser[string]) error {
	switch token := p.Next(); token.Type {
	case wordType:
		p.Node(token.Value)
		p.PushState(w)
		return nil
	case TokenTypeEOF:
		return nil
	default:
		panic("unknown type")
	}
}

var (
	errState = errors.New("errState")
	errParse = errors.New("errParse")
)

type lexErrState struct{}

func (e *lexErrState) Run(context.Context, *Lexer) (LexState, error) {
	return nil, errState
}

type parseErrState struct{}

func (e *parseErrState) Run(_ context.Context, p *Parser[string]) error {
	_ = p.Next()
	return errParse
}

func TestLexParse(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		r := runeio.NewReader(strings.NewReader("Hello\nWorld!"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		tokens := make(chan *Token, 1024)
		lexer := NewLexer(r, tokens, &lexWordState{})
		parser := NewParser[string](tokens, &parseWordState{})

		got, err := LexParse(ctx, lexer, parser)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedRoot := newTree(
			&Node[string]{
				Value:  "Hello",
				Line:   1,
				Column: 1,
				Pos:    0,
			},
			&Node[string]{
				Value:  "World!",
				Line:   2,
				Column: 1,
				Pos:    6,
			},
		)

		if diff := cmp.Diff(expectedRoot, got); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})

	// Test when lexer encounters an error.
	t.Run("lexer error", func(t *testing.T) {
		t.Parallel()

		r := runeio.NewReader(strings.NewReader("Hello\nWorld!"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		tokens := make(chan *Token, 1024)
		lexer := NewLexer(r, tokens, &lexErrState{})
		parser := NewParser[string](tokens, &parseErrState{})

		_, got := LexParse(ctx, lexer, parser)
		want := errState
		if diff := cmp.Diff(want, got, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("unexpected error (-want +got):\n%s", diff)
		}
	})

	// Test when parser encounters an error.
	t.Run("parser error", func(t *testing.T) {
		t.Parallel()

		r := runeio.NewReader(strings.NewReader("Hello\nWorld!"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		tokens := make(chan *Token, 1024)
		lexer := NewLexer(r, tokens, &lexWordState{})
		parser := NewParser[string](tokens, &parseErrState{})

		_, got := LexParse(ctx, lexer, parser)
		want := errParse
		if diff := cmp.Diff(want, got, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("unexpected error (-want +got):\n%s", diff)
		}
	})
}
