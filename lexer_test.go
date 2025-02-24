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
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"github.com/ianlewis/runeio"
)

const (
	wordType TokenType = iota + 1
)

type lexWordState struct{}

func (w *lexWordState) Run(_ context.Context, l *Lexer) (LexState, error) {
	rn := l.Peek()
	if unicode.IsSpace(rn) || rn == EOF {
		// NOTE: This can emit empty words.
		l.Emit(wordType)
		// Discard the space
		if !l.Discard() {
			return nil, nil
		}
	}

	l.Advance()

	return w, nil
}

func TestLexer_Peek(t *testing.T) {
	t.Parallel()

	l := NewLexer(runeio.NewReader(strings.NewReader("Hello\nWorld!")), nil, &lexWordState{})

	rn := l.Peek()
	if err := l.Err(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got, want := rn, 'H'; got != want {
		t.Errorf("Peek: want: %v, got: %v", want, got)
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

	if got, want := l.startPos, 0; got != want {
		t.Errorf("startPos: want: %v, got: %v", want, got)
	}

	if got, want := l.startLine, 0; got != want {
		t.Errorf("startLine: want: %v, got: %v", want, got)
	}

	if got, want := l.startColumn, 0; got != want {
		t.Errorf("startColumn: want: %v, got: %v", want, got)
	}
}

func TestLexer_PeekN(t *testing.T) {
	t.Parallel()

	l := NewLexer(runeio.NewReader(strings.NewReader("Hello\nWorld!")), nil, &lexWordState{})

	rns := l.PeekN(6)
	if err := l.Err(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got, want := string(rns), "Hello\n"; got != want {
		t.Errorf("Peek: want: %q, got: %q", want, got)
	}

	rns = l.PeekN(16)
	if err := l.Err(); err != nil {
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

	if got, want := l.startPos, 0; got != want {
		t.Errorf("startPos: want: %v, got: %v", want, got)
	}

	if got, want := l.startLine, 0; got != want {
		t.Errorf("startLine: want: %v, got: %v", want, got)
	}

	if got, want := l.startColumn, 0; got != want {
		t.Errorf("startColumn: want: %v, got: %v", want, got)
	}
}

func TestLexer_Advance(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Advance!")), nil, &lexWordState{})

		advanced := l.Advance()
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := advanced, true; got != want {
			t.Errorf("Advance: want: %v, got: %v", want, got)
		}

		rns := l.PeekN(10)
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := string(rns), "ello\n!Adva"; got != want {
			t.Errorf("PeekN: want: %q, got: %q", want, got)
		}

		if got, want := l.Pos(), 1; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 0; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 1; got != want {
			t.Errorf("Line: want: %v, got: %v", want, got)
		}

		if got, want := l.Column(), 2; got != want {
			t.Errorf("Column: want: %v, got: %v", want, got)
		}

		if got, want := l.Width(), 1; got != want {
			t.Errorf("Width: want: %q, got: %q", want, got)
		}

		if got, want := l.Token(), "H"; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("")), nil, &lexWordState{})

		advanced := l.Advance()
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := advanced, false; got != want {
			t.Errorf("Advance: want: %v, got: %v", want, got)
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

		if got, want := l.Width(), 0; got != want {
			t.Errorf("Width: want: %q, got: %q", want, got)
		}

		if got, want := l.Token(), ""; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})
}

func TestLexer_AdvanceN(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Advance!")), nil, &lexWordState{})

		advanced := l.AdvanceN(5)
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := advanced, 5; got != want {
			t.Errorf("Advance: want: %v, got: %v", want, got)
		}

		rns := l.PeekN(10)
		if err := l.Err(); err != nil {
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

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Advance!")), nil, &lexWordState{})

		advanced := l.AdvanceN(16)
		if err := l.Err(); err != nil {
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

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Advance!")), nil, &lexWordState{})

		discarded := l.Discard()
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := discarded, true; got != want {
			t.Errorf("Discard: want: %v, got: %v", want, got)
		}

		rns := l.PeekN(10)
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := string(rns), "ello\n!Adva"; got != want {
			t.Errorf("PeekN: want: %q, got: %q", want, got)
		}

		if got, want := l.Pos(), 1; got != want {
			t.Errorf("Pos: want: %v, got: %v", want, got)
		}

		if got, want := l.Cursor(), 1; got != want {
			t.Errorf("Cursor: want: %v, got: %v", want, got)
		}

		if got, want := l.Line(), 1; got != want {
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

	t.Run("failure", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("")), nil, &lexWordState{})

		discarded := l.Discard()
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := discarded, false; got != want {
			t.Errorf("Discard: want: %v, got: %v", want, got)
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

		if got, want := l.Width(), 0; got != want {
			t.Errorf("Width: want: %q, got: %q", want, got)
		}

		if got, want := l.Token(), ""; got != want {
			t.Errorf("Token: want: %q, got: %q", want, got)
		}
	})
}

func TestLexer_DiscardN(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Discard!")), nil, &lexWordState{})

		discarded := l.DiscardN(7)
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := discarded, 7; got != want {
			t.Errorf("Discard: want: %v, got: %v", want, got)
		}

		rns := l.PeekN(8)
		if err := l.Err(); err != nil {
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

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Discard!")), nil, &lexWordState{})

		discarded := l.DiscardN(16)
		if err := l.Err(); err != nil {
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

func TestLexer_Find_match(t *testing.T) {
	t.Parallel()

	l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Find!")), nil, &lexWordState{})

	token := l.Find([]string{"Find"})
	if err := l.Err(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got, want := token, "Find"; got != want {
		t.Errorf("unexpected token: want: %q, got: %q", want, got)
	}

	rns := l.PeekN(5)
	if err := l.Err(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	//nolint:goconst // It's easier to understand the test if the string is written out.
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

	//nolint:goconst // It's easier to understand the test if the string is written out.
	if got, want := l.Token(), "Hello\n!"; got != want {
		t.Errorf("Token: want: %q, got: %q", want, got)
	}
}

func TestLexer_Find_short_match(t *testing.T) {
	t.Parallel()

	l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Find!")), nil, &lexWordState{})

	token := l.Find([]string{"no match", "Find!"})
	if err := l.Err(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got, want := token, "Find!"; got != want {
		t.Errorf("unexpected token: want: %q, got: %q", want, got)
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
}

func TestLexer_Find_no_match(t *testing.T) {
	t.Parallel()

	l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Find!")), nil, &lexWordState{})

	token := l.Find([]string{"no match"})
	if err := l.Err(); err != nil {
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
}

func TestLexer_Ignore(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Ignore!\n")), nil, &lexWordState{})

		advanced := l.AdvanceN(7)
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := advanced, 7; got != want {
			t.Errorf("Advance: want: %v, got: %v", want, got)
		}

		rns := l.PeekN(7)
		if err := l.Err(); err != nil {
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

		advanced = l.AdvanceN(7)
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := advanced, 7; got != want {
			t.Errorf("Advance: want: %v, got: %v", want, got)
		}

		rns = l.PeekN(1)
		if err := l.Err(); err != nil {
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

func TestLexer_DiscardTo(t *testing.T) {
	t.Parallel()

	t.Run("match", func(t *testing.T) {
		t.Parallel()

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Find!")), nil, &lexWordState{})

		token := l.DiscardTo([]string{"Find"})
		if err := l.Err(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if got, want := token, "Find"; got != want {
			t.Errorf("unexpected token: want: %q, got: %q", want, got)
		}

		rns := l.PeekN(5)
		if err := l.Err(); err != nil {
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

		l := NewLexer(runeio.NewReader(strings.NewReader("Hello\n!Find!")), nil, &lexWordState{})

		token := l.DiscardTo([]string{"no match"})
		if err := l.Err(); err != nil {
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

func TestLexer_tokens(t *testing.T) {
	t.Parallel()

	tokens := make(chan *Token, 1024)
	l := NewLexer(runeio.NewReader(strings.NewReader("Hello tokens!")), tokens, &lexWordState{})
	if err := l.Lex(context.Background()); err != nil {
		t.Errorf("unexpected error %v", err)
	}

	var items []*Token
	for item := range tokens {
		items = append(items, item)
	}
	got := items
	want := []*Token{
		{
			Type:   wordType,
			Value:  "Hello",
			Pos:    0,
			Line:   1,
			Column: 1,
		},
		{
			Type:   wordType,
			Value:  "tokens!",
			Pos:    6,
			Line:   1,
			Column: 7,
		},
		&tokenEOF,
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected output (-want +got):\n%s", diff)
	}
}
