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

	"github.com/ianlewis/lexparse/lexer"
)

type parseTokenState struct{}

func (w *parseTokenState) Run(ctx context.Context, p *Parser[string]) error {
	switch token := p.Next(ctx); token.Type {
	case lexer.TokenTypeIdent:
		p.Node(token.Value)
		p.PushState(w)

		return nil
	case lexer.TokenTypeEOF:
		return nil
	default:
		panic("unknown type")
	}
}

func TestScannerLexParse(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		r := strings.NewReader("Hello\nWorld")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		l := lexer.NewScanningLexer(r)

		got, err := LexParse(ctx, l, &parseTokenState{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedRoot := addParent(
			&Node[string]{
				Start: lexer.Position{
					Offset: 0,
					Line:   1,
					Column: 1,
				},
				Children: []*Node[string]{
					{
						Value: "Hello",
						Start: lexer.Position{
							Offset: 0,
							Line:   1,
							Column: 1,
						},
					},
					{
						Value: "World",
						Start: lexer.Position{
							Offset: 6,
							Line:   2,
							Column: 1,
						},
					},
				},
			},
		)

		if diff := cmp.Diff(expectedRoot, got); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})
}

type parseWordState struct{}

func (w *parseWordState) Run(ctx context.Context, p *Parser[string]) error {
	switch token := p.Next(ctx); token.Type {
	case wordType:
		p.Node(token.Value)
		p.PushState(w)

		return nil
	case lexer.TokenTypeEOF:
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

//nolint:ireturn // returning interface is required to satisfy lexer.LexState.
func (e *lexErrState) Run(context.Context, *lexer.CustomLexer) (lexer.LexState, error) {
	return nil, errState
}

type parseErrState struct{}

func (e *parseErrState) Run(ctx context.Context, p *Parser[string]) error {
	_ = p.Next(ctx)
	return errParse
}

func TestCustomLexParse(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		r := strings.NewReader("Hello\nWorld!")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		l := lexer.NewCustomLexer(r, &lexWordState{})

		got, err := LexParse(ctx, l, &parseWordState{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedRoot := addParent(
			&Node[string]{
				Start: lexer.Position{
					Offset: 0,
					Line:   1,
					Column: 1,
				},
				Children: []*Node[string]{
					{
						Value: "Hello",
						Start: lexer.Position{
							Offset: 0,
							Line:   1,
							Column: 1,
						},
					},
					{
						Value: "World!",
						Start: lexer.Position{
							Offset: 6,
							Line:   2,
							Column: 1,
						},
					},
				},
			},
		)

		if diff := cmp.Diff(expectedRoot, got); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})

	// Test when lexer encounters an error.
	t.Run("lexer error", func(t *testing.T) {
		t.Parallel()

		r := strings.NewReader("Hello\nWorld!")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		l := lexer.NewCustomLexer(r, &lexErrState{})
		_, got := LexParse(ctx, l, &parseErrState{})

		want := errState
		if diff := cmp.Diff(want, got, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("unexpected error (-want +got):\n%s", diff)
		}
	})

	// Test when parser encounters an error.
	t.Run("parser error", func(t *testing.T) {
		t.Parallel()

		r := strings.NewReader("Hello\nWorld!")

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		l := lexer.NewCustomLexer(r, &lexWordState{})
		_, got := LexParse(ctx, l, &parseErrState{})

		want := errParse
		if diff := cmp.Diff(want, got, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("unexpected error (-want +got):\n%s", diff)
		}
	})
}
