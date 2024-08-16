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
	errSymbol = errors.New("invalid symbol")
)

type nodeType int

const (
	textNodeType nodeType = iota
	actionNodeType
)

type tmplNode struct {
	typ    nodeType
	action string
	text   string
}

// stateText parses normal text.
func stateText(_ context.Context, l *lexparse.Lexer) (lexparse.State, error) {
	// Search the input for left brackets.
	token, err := l.Find([]string{actionLeft})

	// Emit the text up until this point.
	lexeme := l.Lexeme(textType)
	if lexeme.Value != "" {
		l.Emit(lexeme)
	}

	// Progress to lexing the action if brackets are found.
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

	// Process the token if the right bracket is found.
	var nextState lexparse.State
	if token == actionRight {
		// Emit the lexeme.
		lexeme := l.Lexeme(actionType)
		if strings.TrimSpace(lexeme.Value) != "" {
			l.Emit(lexeme)
		}

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
func parseInit(_ context.Context, p *lexparse.Parser[*tmplNode]) (lexparse.ParseFn[*tmplNode], error) {
	l := p.Peek()
	if l == nil {
		return nil, nil
	}

	switch l.Type {
	case textType:
		return parseText, nil
	case actionType:
		return parseAction, nil
	default:
		return nil, fmt.Errorf("%w: %v", errType, l.Type)
	}
}

// parseText handles normal text.
func parseText(_ context.Context, p *lexparse.Parser[*tmplNode]) (lexparse.ParseFn[*tmplNode], error) {
	// Get the next lexeme from the parser.
	l := p.Next()
	if l == nil {
		return nil, nil
	}
	// Emit a text node.
	p.Node(&tmplNode{
		typ:  textNodeType,
		text: l.Value,
	})

	// Return to the init state.
	return parseInit, nil
}

// parseAction handles replacement actions (e.g. {{ var }}).
func parseAction(_ context.Context, p *lexparse.Parser[*tmplNode]) (lexparse.ParseFn[*tmplNode], error) {
	// Get the next lexeme from the parser.
	l := p.Next()
	if l == nil {
		return nil, nil
	}

	// Emit an action (variable) node.
	p.Node(&tmplNode{
		typ:    actionNodeType,
		action: strings.TrimSpace(l.Value),
	})

	// Return to the init state.
	return parseInit, nil
}

// execute executes the template with the given data.
func execute(root *lexparse.Node[*tmplNode], data map[string]string) (string, error) {
	var b strings.Builder
	for _, n := range root.Children {
		switch n.Value.typ {
		case textNodeType:
			// Write raw text to the output.
			b.WriteString(n.Value.text)
		case actionNodeType:
			// Replace templated variables with given data.
			val, ok := data[n.Value.action]
			if !ok {
				return b.String(), fmt.Errorf("%w: %q", errSymbol, n.Value.action)
			}
			b.WriteString(val)
		}
	}
	return b.String(), nil
}

// ExampleLexParse implements a simple templating language.
func ExampleLexParse() {
	r := runeio.NewReader(strings.NewReader("Hello {{ subject }}!"))
	t, err := lexparse.LexParse(context.Background(), r, lexparse.StateFn(stateText), parseInit)
	if err != nil {
		panic(err)
	}
	txt, err := execute(t, map[string]string{"subject": "World"})
	if err != nil {
		panic(err)
	}
	fmt.Print(txt)

	// Output: Hello World!
}
