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
	"io"
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"github.com/ianlewis/runeio"
)

const (
	unusedType LexemeType = iota
	wordType
)

type wordState struct{}

func (w *wordState) Run(_ context.Context, l *Lexer) (State, error) {
	rn, err := l.Peek(1)
	if errors.Is(err, io.EOF) || (err == nil && unicode.IsSpace(rn[0])) {
		// NOTE: This can emit empty words.
		l.Emit(wordType)
		// Discard the space
		if _, dErr := l.Discard(len(rn)); dErr != nil {
			return nil, dErr
		}
	}
	if err != nil {
		return nil, err
	}

	if _, aErr := l.Advance(len(rn)); aErr != nil {
		return nil, aErr
	}

	return w, nil
}

func TestLexer_Peek(t *testing.T) {
	t.Parallel()

	l := NewLexer(runeio.NewReader(strings.NewReader("Hello\nWorld!")), &wordState{})

	rns, err := l.Peek(6)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got, want := string(rns), "Hello\n"; got != want {
		t.Errorf("Peek: want: %q, got: %q", want, got)
	}

	rns, err = l.Peek(16)
	if !errors.Is(err, io.EOF) {
		t.Errorf("unexpected error: %v", err)
	}
	if got, want := string(rns), "Hello\nWorld!"; got != want {
		t.Errorf("Peek: want: %q, got: %q", want, got)
	}

	if got, want := l.Pos(), 0; got != want {
		t.Errorf("Pos: want: %v, got: %v", want, got)
	}

	if got, want := l.Cursor(), 0; got != want {
		t.Errorf("Cursor: want: %v, got: %v", want, got)
	}

	if got, want := l.Line(), 1; got != want {
		t.Errorf("Line: want: %v, got: %v", want, got)
	}

	if got, want := l.Column(), 1; got != want {
		t.Errorf("Column: want: %v, got: %v", want, got)
	}

	if got, want := l.s.startPos, 0; got != want {
		t.Errorf("startPos: want: %v, got: %v", want, got)
	}

	if got, want := l.s.startLine, 0; got != want {
		t.Errorf("startLine: want: %v, got: %v", want, got)
	}

	if got, want := l.s.startColumn, 0; got != want {
		t.Errorf("startColumn: want: %v, got: %v", want, got)
	}
}

func TestLexer_Advance(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Advance!")), &wordState{})

		advanced, err := l.Advance(5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := advanced, 5; got != want {
			t.Errorf("Advance: want: %v, got: %v", want, got)
		}

		rns, err := l.Peek(10)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := string(rns), "\n!Advance!"; got != want {
			t.Errorf("Peek: want: %q, got: %q", want, got)
		}

		if got, want := l.Pos(), 5; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 0; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 1; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 6; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 5; got != want {
			t.Errorf("Width: want: %q, got: %q", want, got)
		}

		if got, want := l.Token(), "Hello"; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})

	t.Run("past end", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Advance!")), &wordState{})

		advanced, err := l.Advance(16)
		if !errors.Is(err, io.EOF) {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := advanced, 15; got != want {
			t.Errorf("Advance: want: %v, got: %v", want, got)
		}

		if got, want := l.Pos(), 15; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 0; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 2; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 10; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 15; got != want {
			t.Errorf("Width: want: %v, got: %v", want, got)
		}

		if got, want := l.Token(), "Hello\n!Advance!"; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})
}

func TestLexer_Discard(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Discard!")), &wordState{})

		discarded, err := l.Discard(7)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := discarded, 7; got != want {
			t.Errorf("Discard: want: %v, got: %v", want, got)
		}

		rns, err := l.Peek(8)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := string(rns), "Discard!"; got != want {
			t.Errorf("Peek: want: %q, got: %q", want, got)
		}

		if got, want := l.Pos(), 7; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 7; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 2; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 2; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 0; got != want {
			t.Errorf("Width: want: %v, got: %v", want, got)
		}

		if got, want := l.Token(), ""; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})

	t.Run("past end", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Discard!")), &wordState{})

		discarded, err := l.Discard(16)
		if !errors.Is(err, io.EOF) {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := discarded, 15; got != want {
			t.Errorf("Discard: want: %v, got: %v", want, got)
		}

		if got, want := l.Pos(), 15; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 15; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 2; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 10; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 0; got != want {
			t.Errorf("Width: want: %v, got: %v", want, got)
		}

		if got, want := l.Token(), ""; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})
}

func TestLexer_Find(t *testing.T) {
	t.Parallel()

	t.Run("match", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Find!")), &wordState{})

		token, err := l.Find([]string{"Find"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := token, "Find"; got != want {
			t.Errorf("unexpected token: want: %q, got: %q", want, got)
		}

		rns, err := l.Peek(5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := string(rns), "Find!"; got != want {
			t.Errorf("Peek: want: %q, got: %q", want, got)
		}

		if got, want := l.Pos(), 7; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 0; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 2; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 2; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 7; got != want {
			t.Errorf("Width: want: %v, got: %v", want, got)
		}

		if got, want := l.Token(), "Hello\n!"; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})

	t.Run("no match", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Find!")), &wordState{})

		token, err := l.Find([]string{"no match"})
		if !errors.Is(err, io.EOF) {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := token, ""; got != want {
			t.Errorf("unexpected token: want: %q, got: %q", want, got)
		}

		if got, want := l.Pos(), 12; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 0; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 2; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 7; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 12; got != want {
			t.Errorf("Width: want: %v, got: %v", want, got)
		}

		if got, want := l.Token(), "Hello\n!Find!"; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})
}

func TestLexer_Ignore(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Ignore!\n")), &wordState{})

		advanced, err := l.Advance(7)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := advanced, 7; got != want {
			t.Errorf("Advance: want: %v, got: %v", want, got)
		}

		rns, err := l.Peek(7)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := string(rns), "Ignore!"; got != want {
			t.Errorf("Peek: want: %q, got: %q", want, got)
		}

		if got, want := l.Pos(), 7; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 0; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 2; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 2; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 7; got != want {
			t.Errorf("Width: want: %v, got: %v", want, got)
		}

		if got, want := l.Token(), "Hello\n!"; got != want {
			t.Errorf("Token: want: %v, got: %v", want, got)
		}

		l.Ignore()

		advanced, err = l.Advance(7)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := advanced, 7; got != want {
			t.Errorf("Advance: want: %v, got: %v", want, got)
		}

		rns, err = l.Peek(1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := string(rns), "\n"; got != want {
			t.Errorf("Peek: want: %q, got: %q", want, got)
		}

		if got, want := l.Pos(), 14; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 7; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 2; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 9; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 7; got != want {
			t.Errorf("Width: want: %v, got: %v", want, got)
		}

		if got, want := l.Token(), "Ignore!"; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})
}

func TestLexer_SkipTo(t *testing.T) {
	t.Parallel()

	t.Run("match", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Find!")), &wordState{})

		token, err := l.SkipTo([]string{"Find"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := token, "Find"; got != want {
			t.Errorf("unexpected token: want: %q, got: %q", want, got)
		}

		rns, err := l.Peek(5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := string(rns), "Find!"; got != want {
			t.Errorf("Peek: want: %q, got: %q", want, got)
		}

		if got, want := l.Pos(), 7; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 7; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 2; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 2; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 0; got != want {
			t.Errorf("Width: want: %q, got: %q", want, got)
		}

		if got, want := l.Token(), ""; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})

	t.Run("no match", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Find!")), &wordState{})

		token, err := l.SkipTo([]string{"no match"})
		if !errors.Is(err, io.EOF) {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := token, ""; got != want {
			t.Errorf("unexpected token: want: %q, got: %q", want, got)
		}

		if got, want := l.Pos(), 12; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 12; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 2; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 7; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 0; got != want {
			t.Errorf("Width: want: %v, got: %v", want, got)
		}

		if got, want := l.Token(), ""; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})
}

func TestLexer_lexemes(t *testing.T) {
	t.Parallel()

	l := NewLexer(runeio.NewReader(strings.NewReader("Hello Lexemes!")), &wordState{})
	lexemes := l.Lex(context.Background())

	var items []*Lexeme
	for item := range lexemes {
		items = append(items, item)
	}
	got := items
	want := []*Lexeme{
		{
			Type:   wordType,
			Value:  "Hello",
			Pos:    0,
			Line:   1,
			Column: 1,
		},
		{
			Type:   wordType,
			Value:  "Lexemes!",
			Pos:    6,
			Line:   1,
			Column: 7,
		},
	}
	err := l.Err()
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected output (-want +got):\n%s", diff)
	}
}
