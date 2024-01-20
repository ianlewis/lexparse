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

package lexparse_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ianlewis/runeio"

	"github.com/ianlewis/lexparse"
)

const (
	textType lexparse.LexemeType = iota
	actionType
)

const (
	actionLeft  = "{{"
	actionRight = "}}"
)

var (
	errType   = errors.New("unexpected type")
	errSymbol = errors.New("missing symbol")
)

// stateText parses normal text.
func stateText(_ context.Context, l *lexparse.Lexer) (lexparse.State, error) {
	token, err := l.Find([]string{actionLeft})
	lexeme := l.Lexeme(textType)
	if lexeme.Value != "" {
		l.Emit(lexeme)
	}

	var nextState lexparse.State
	if token == actionLeft {
		nextState = lexparse.StateFn(stateAction)
	}

	if err != nil {
		return nextState, fmt.Errorf("lexing text: %w", err)
	}
	return nextState, nil
}

// stateAction lexes replacement actions (e.g. {{ var }}).
func stateAction(_ context.Context, l *lexparse.Lexer) (lexparse.State, error) {
	// Discard the left brackets
	if _, err := l.Discard(len(actionLeft)); err != nil {
		return nil, fmt.Errorf("lexing action: %w", err)
	}

	// Find the right brackets.
	token, err := l.Find([]string{actionRight})
	lexeme := l.Lexeme(actionType)
	if lexeme.Value != "" {
		l.Emit(lexeme)
	}

	var nextState lexparse.State
	if token == actionRight {
		// Discard the right brackets
		if _, errDiscard := l.Discard(len(actionRight)); errDiscard != nil {
			return nil, fmt.Errorf("lexing action: %w", errDiscard)
		}
		nextState = lexparse.StateFn(stateText)
	}

	if err != nil {
		return nextState, fmt.Errorf("lexing action: %w", err)
	}
	return nextState, nil
}

// parseInit delegates to another parse function based on lexeme type.
func parseInit(data map[string]string) lexparse.ParseFn[string] {
	return func(ctx context.Context, p *lexparse.Parser[string]) (lexparse.ParseFn[string], error) {
		l := p.Peek()
		if l == nil {
			return nil, nil
		}

		switch l.Type {
		case textType:
			return parseText(data), nil
		case actionType:
			return parseAction(data), nil
		default:
			return nil, fmt.Errorf("%w: %v", errType, l.Type)
		}
	}
}

// parseText handles normal text.
func parseText(data map[string]string) lexparse.ParseFn[string] {
	return func(ctx context.Context, p *lexparse.Parser[string]) (lexparse.ParseFn[string], error) {
		l := p.Next()
		if l == nil {
			return nil, nil
		}
		p.Node(l.Value)
		return parseInit(data), nil
	}
}

// parseAction handles replacement actions (e.g. {{ var }}).
func parseAction(data map[string]string) lexparse.ParseFn[string] {
	return func(ctx context.Context, p *lexparse.Parser[string]) (lexparse.ParseFn[string], error) {
		l := p.Next()
		if l == nil {
			return nil, nil
		}
		v, ok := data[strings.TrimSpace(l.Value)]
		if !ok {
			return nil, fmt.Errorf("%w: %q", errSymbol, l.Value)
		}
		p.Node(v)
		return parseInit(data), nil
	}
}

// ExampleLexParse implements a simple templating language.
func ExampleLexParse() {
	r := runeio.NewReader(strings.NewReader("Hello {{ subject }}!"))
	data := map[string]string{"subject": "World"}
	t, err := lexparse.LexParse(context.Background(), r, lexparse.StateFn(stateText), parseInit(data))
	if err != nil {
		panic(err)
	}

	for _, n := range t.Root.Children {
		fmt.Print(n.Value)
	}
	// Output: Hello World!
}
