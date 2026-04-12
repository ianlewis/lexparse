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
	"context"
	"io"
	"strings"
	"testing"
	"unicode"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

const (
	wordType TokenType = iota + 1
)

type lexWordState struct{}

//nolint:ireturn // Returning interface required to satisfy [LexState.Run]
func (w *lexWordState) Run(_ context.Context, c *CustomLexerCursor) (LexState, error) {
	rn := c.Peek()
	if unicode.IsSpace(rn) || rn == EOF {
		// NOTE: This can emit empty words.
		c.Emit(wordType)
		// Discard the space
		if !c.Discard() {
			return nil, io.EOF
		}
	}

	c.Advance()

	return w, nil
}

func TestCustomLexerCursor_Peek(t *testing.T) {
	t.Parallel()

	cursor := NewCustomLexerCursor(
		NewCustomLexer(strings.NewReader("Hello\nWorld!"), &lexWordState{}),
	)

	rn := cursor.Peek()

	if diff := cmp.Diff('H', rn); diff != "" {
		t.Errorf("Peek (-want +got):\n%s", diff)
	}

	expectedPos := Position{
		Offset: 0,
		Line:   1,
		Column: 1,
	}

	if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
		t.Errorf("Pos (-want +got):\n%s", diff)
	}

	expectedCursor := Position{
		Offset: 0,
		Line:   1,
		Column: 1,
	}

	if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
		t.Errorf("Cursor (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
		t.Errorf("Err (-want +got):\n%s", diff)
	}
}

func TestCustomLexerCursor_PeekN(t *testing.T) {
	t.Parallel()

	cursor := NewCustomLexerCursor(
		NewCustomLexer(strings.NewReader("Hello\nWorld!"), &lexWordState{}),
	)

	if diff := cmp.Diff("Hello\n", string(cursor.PeekN(6))); diff != "" {
		t.Errorf("PeekN (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff("Hello\nWorld!", string(cursor.PeekN(16))); diff != "" {
		t.Errorf("PeekN (-want +got):\n%s", diff)
	}

	expectedPos := Position{
		Offset: 0,
		Line:   1,
		Column: 1,
	}

	if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
		t.Errorf("Pos (-want +got):\n%s", diff)
	}

	expectedCursor := Position{
		Offset: 0,
		Line:   1,
		Column: 1,
	}

	if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
		t.Errorf("Cursor (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
		t.Errorf("Err (-want +got):\n%s", diff)
	}
}

//nolint:dupl // Tests are testing different methods.
func TestCustomLexerCursor_Advance(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader("Hello\n!Advance!"), &lexWordState{}),
		)

		if diff := cmp.Diff(true, cursor.Advance()); diff != "" {
			t.Errorf("Advance (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("ello\n!Adva", string(cursor.PeekN(10))); diff != "" {
			t.Errorf("PeekN (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 1,
			Line:   1,
			Column: 2,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(1, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("H", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})

	t.Run("end of input", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader(""), &lexWordState{}),
		)

		if diff := cmp.Diff(false, cursor.Advance()); diff != "" {
			t.Errorf("Advance (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(0, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})
}

func TestCustomLexerCursor_AdvanceN(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader("Hello\n!Advance!"), &lexWordState{}),
		)

		if diff := cmp.Diff(5, cursor.AdvanceN(5)); diff != "" {
			t.Errorf("AdvanceN (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("\n!Advance!", string(cursor.PeekN(10))); diff != "" {
			t.Errorf("PeekN (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 5,
			Line:   1,
			Column: 6,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(5, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("Hello", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})

	t.Run("past end", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader("Hello\n!Advance!"), &lexWordState{}),
		)

		if diff := cmp.Diff(15, cursor.AdvanceN(16)); diff != "" {
			t.Errorf("AdvanceN (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 15,
			Line:   2,
			Column: 10,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(15, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("Hello\n!Advance!", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})
}

//nolint:dupl // Tests are testing different methods.
func TestCustomLexerCursor_Discard(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader("Hello\n!Advance!"), &lexWordState{}),
		)

		if diff := cmp.Diff(true, cursor.Discard()); diff != "" {
			t.Errorf("Discard (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("ello\n!Adva", string(cursor.PeekN(10))); diff != "" {
			t.Errorf("PeekN (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 1,
			Line:   1,
			Column: 2,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 1,
			Line:   1,
			Column: 2,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(0, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})

	t.Run("end of input", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader(""), &lexWordState{}),
		)

		if diff := cmp.Diff(false, cursor.Discard()); diff != "" {
			t.Errorf("Discard (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(0, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})
}

func TestCustomLexerCursor_DiscardN(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader("Hello\n!Discard!"), &lexWordState{}),
		)

		if diff := cmp.Diff(7, cursor.DiscardN(7)); diff != "" {
			t.Errorf("DiscardN (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("Discard!", string(cursor.PeekN(8))); diff != "" {
			t.Errorf("PeekN (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 7,
			Line:   2,
			Column: 2,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 7,
			Line:   2,
			Column: 2,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(0, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})

	t.Run("past end", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader("Hello\n!Discard!"), &lexWordState{}),
		)

		if diff := cmp.Diff(15, cursor.DiscardN(16)); diff != "" {
			t.Errorf("DiscardN (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 15,
			Line:   2,
			Column: 10,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 15,
			Line:   2,
			Column: 10,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(0, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})
}

//nolint:dupl // Similar to TestLexer_DiscardTo/match
func TestCustomLexerCursor_Find_match(t *testing.T) {
	t.Parallel()

	cursor := NewCustomLexerCursor(
		NewCustomLexer(strings.NewReader("Hello\n!Find!"), &lexWordState{}),
	)

	if diff := cmp.Diff("Find", cursor.Find([]string{"Find"})); diff != "" {
		t.Errorf("Find (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff("Find!", string(cursor.PeekN(5))); diff != "" {
		t.Errorf("PeekN (-want +got):\n%s", diff)
	}

	expectedPos := Position{
		Offset: 7,
		Line:   2,
		Column: 2,
	}

	if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
		t.Errorf("Pos (-want +got):\n%s", diff)
	}

	expectedCursor := Position{
		Offset: 0,
		Line:   1,
		Column: 1,
	}

	if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
		t.Errorf("Cursor (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(7, cursor.Width()); diff != "" {
		t.Errorf("Width (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff("Hello\n!", cursor.Token()); diff != "" {
		t.Errorf("Token (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
		t.Errorf("Err (-want +got):\n%s", diff)
	}
}

func TestCustomLexerCursor_Find_short_match(t *testing.T) {
	t.Parallel()

	cursor := NewCustomLexerCursor(
		NewCustomLexer(strings.NewReader("Hello\n!Find!"), &lexWordState{}),
	)

	if diff := cmp.Diff("Find!", cursor.Find([]string{"no match", "Find!"})); diff != "" {
		t.Errorf("Find (-want +got):\n%s", diff)
	}

	expectedPos := Position{
		Offset: 7,
		Line:   2,
		Column: 2,
	}

	if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
		t.Errorf("Pos (-want +got):\n%s", diff)
	}

	expectedCursor := Position{
		Offset: 0,
		Line:   1,
		Column: 1,
	}

	if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
		t.Errorf("Cursor (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(7, cursor.Width()); diff != "" {
		t.Errorf("Width (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff("Hello\n!", cursor.Token()); diff != "" {
		t.Errorf("Token (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
		t.Errorf("Err (-want +got):\n%s", diff)
	}
}

//nolint:dupl // Similar to TestLexer_DiscardTo/no_match
func TestCustomLexerCursor_Find_no_match(t *testing.T) {
	t.Parallel()

	cursor := NewCustomLexerCursor(
		NewCustomLexer(strings.NewReader("Hello\n!Find!"), &lexWordState{}),
	)

	if diff := cmp.Diff("", cursor.Find([]string{"no match"})); diff != "" {
		t.Errorf("Find (-want +got):\n%s", diff)
	}

	expectedPos := Position{
		Offset: 12,
		Line:   2,
		Column: 7,
	}

	if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
		t.Errorf("Pos (-want +got):\n%s", diff)
	}

	expectedCursor := Position{
		Offset: 0,
		Line:   1,
		Column: 1,
	}

	if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
		t.Errorf("Cursor (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(12, cursor.Width()); diff != "" {
		t.Errorf("Width (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff("Hello\n!Find!", cursor.Token()); diff != "" {
		t.Errorf("Token (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
		t.Errorf("Err (-want +got):\n%s", diff)
	}
}

func TestCustomLexerCursor_Ignore(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader("Hello\n!Ignore!\n"), &lexWordState{}),
		)

		if diff := cmp.Diff(7, cursor.AdvanceN(7)); diff != "" {
			t.Errorf("AdvanceN (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("Ignore!", string(cursor.PeekN(7))); diff != "" {
			t.Errorf("PeekN (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 7,
			Line:   2,
			Column: 2,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(7, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("Hello\n!", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		cursor.Ignore()

		if diff := cmp.Diff(7, cursor.AdvanceN(7)); diff != "" {
			t.Errorf("AdvanceN (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("\n", string(cursor.PeekN(1))); diff != "" {
			t.Errorf("PeekN (-want +got):\n%s", diff)
		}

		expectedPos = Position{
			Offset: 14,
			Line:   2,
			Column: 9,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor = Position{
			Offset: 7,
			Line:   2,
			Column: 2,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(7, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("Ignore!", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})
}

func TestCustomLexerCursor_DiscardTo(t *testing.T) {
	t.Parallel()

	//nolint:dupl // Similar to TestLexer_Find_match
	t.Run("match", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader("Hello\n!Find!"), &lexWordState{}),
		)

		if diff := cmp.Diff("Find", cursor.DiscardTo([]string{"Find"})); diff != "" {
			t.Errorf("DiscardTo (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("Find!", string(cursor.PeekN(5))); diff != "" {
			t.Errorf("PeekN (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 7,
			Line:   2,
			Column: 2,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 7,
			Line:   2,
			Column: 2,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(0, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})

	//nolint:dupl // Similar to TestLexer_Find_no_match
	t.Run("no match", func(t *testing.T) {
		t.Parallel()

		cursor := NewCustomLexerCursor(
			NewCustomLexer(strings.NewReader("Hello\n!Find!"), &lexWordState{}),
		)

		if diff := cmp.Diff("", cursor.DiscardTo([]string{"no match"})); diff != "" {
			t.Errorf("DiscardTo (-want +got):\n%s", diff)
		}

		expectedPos := Position{
			Offset: 12,
			Line:   2,
			Column: 7,
		}

		if diff := cmp.Diff(expectedPos, cursor.Pos()); diff != "" {
			t.Errorf("Pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset: 12,
			Line:   2,
			Column: 7,
		}

		if diff := cmp.Diff(expectedCursor, cursor.Cursor()); diff != "" {
			t.Errorf("Cursor (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(0, cursor.Width()); diff != "" {
			t.Errorf("Width (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff("", cursor.Token()); diff != "" {
			t.Errorf("Token (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, cursor.l.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})
}

func TestCustomLexer_NextToken(t *testing.T) {
	t.Parallel()

	t.Run("parsing", func(t *testing.T) {
		t.Parallel()

		customLexer := NewCustomLexer(strings.NewReader("Hello World!"), &lexWordState{})

		ctx := context.Background()

		expectedToken := &Token{
			Type:  wordType,
			Value: "Hello",
			Start: Position{
				Offset: 0,
				Line:   1,
				Column: 1,
			},
			End: Position{
				Offset: 5,
				Line:   1,
				Column: 6,
			},
		}

		if diff := cmp.Diff(expectedToken, customLexer.NextToken(ctx)); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, customLexer.Err()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}

		expectedToken2 := &Token{
			Type:  wordType,
			Value: "World!",
			Start: Position{
				Offset: 6,
				Line:   1,
				Column: 7,
			},
			End: Position{
				Offset: 12,
				Line:   1,
				Column: 13,
			},
		}

		if diff := cmp.Diff(expectedToken2, customLexer.NextToken(ctx)); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, customLexer.Err()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}

		expectedToken3 := &Token{
			Type:  TokenTypeEOF,
			Value: "",
			Start: Position{
				Offset: 12,
				Line:   1,
				Column: 13,
			},
			End: Position{
				Offset: 12,
				Line:   1,
				Column: 13,
			},
		}

		if diff := cmp.Diff(expectedToken3, customLexer.NextToken(ctx)); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}

		if diff := cmp.Diff(nil, customLexer.Err(), cmpopts.EquateErrors()); diff != "" {
			t.Errorf("Err (-want +got):\n%s", diff)
		}
	})
}

func TestCustomLexer_SetFilename(t *testing.T) {
	t.Parallel()

	t.Run("SetFilename", func(t *testing.T) {
		t.Parallel()

		customLexer := NewCustomLexer(strings.NewReader("Hello World!"), &lexWordState{})
		customLexer.SetFilename("testfile.txt")

		expectedPos := Position{
			Offset:   0,
			Line:     1,
			Column:   1,
			Filename: "testfile.txt",
		}

		if diff := cmp.Diff(expectedPos, customLexer.pos); diff != "" {
			t.Errorf("pos (-want +got):\n%s", diff)
		}

		expectedCursor := Position{
			Offset:   0,
			Line:     1,
			Column:   1,
			Filename: "testfile.txt",
		}

		if diff := cmp.Diff(expectedCursor, customLexer.cursor); diff != "" {
			t.Errorf("cursor (-want +got):\n%s", diff)
		}
	})
}
