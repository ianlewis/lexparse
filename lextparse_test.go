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

func parseWord(_ context.Context, p *Parser[string]) (ParseFn[string], error) {
	l := p.Next()
	if l == nil {
		return nil, nil
	}
	p.Node(l.Value)
	return parseWord, nil
}

var (
	errState = errors.New("errState")
	errParse = errors.New("errParse")
)

func errStateFn(context.Context, *Lexer) (State, error) {
	return nil, errState
}

func errParseFn(_ context.Context, p *Parser[string]) (ParseFn[string], error) {
	_ = p.Next()
	return nil, errParse
}

func TestLexParse(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		r := runeio.NewReader(strings.NewReader("Hello\nWorld!"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		got, err := LexParse(ctx, r, &wordState{}, parseWord)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		want := &Tree[string]{
			Root: &Node[string]{},
		}
		want.Root.Children = append(want.Root.Children,
			&Node[string]{
				Value:  "Hello",
				Parent: want.Root,
			},
			&Node[string]{
				Value:  "World!",
				Parent: want.Root,
			},
		)

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("unexpected output (-want +got):\n%s", diff)
		}
	})

	// Test when lexer encounters an error.
	t.Run("lexer error", func(t *testing.T) {
		r := runeio.NewReader(strings.NewReader("Hello\nWorld!"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, got := LexParse(ctx, r, StateFn(errStateFn), errParseFn)
		want := errState
		if diff := cmp.Diff(want, got, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("unexpected error (-want +got):\n%s", diff)
		}
	})

	// Test when parser encounters an error.
	t.Run("parser error", func(t *testing.T) {
		r := runeio.NewReader(strings.NewReader("Hello\nWorld!"))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, got := LexParse(ctx, r, &wordState{}, errParseFn)
		want := errParse
		if diff := cmp.Diff(want, got, cmpopts.EquateErrors()); diff != "" {
			t.Errorf("unexpected error (-want +got):\n%s", diff)
		}
	})
}
