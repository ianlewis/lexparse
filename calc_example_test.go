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
	"fmt"
	"strconv"
	"strings"

	"github.com/ianlewis/runeio"

	"github.com/ianlewis/lexparse"
)

const (
	numType lexparse.LexemeType = iota
	openParenType
	closeParenType
	operType
)

// stateExpr is the base state for parsing a postfix expression.
func stateExpr(_ context.Context, l *lexparse.Lexer) (lexparse.State, error) {
	for {
		rn, err := l.Peek(1)
		if err != nil {
			return nil, fmt.Errorf("lexing expression: %w", err)
		}
		switch {
		// Check if digit
		case rn[0] >= 48 && rn[0] <= 57:
			return lexparse.StateFn(stateNum), nil
			// Check if parenthesis
		case rn[0] == '(':
			return lexparse.StateFn(stateOpenParen), nil
		case rn[0] == ')':
			return lexparse.StateFn(stateCloseParen), nil
		case rn[0] == '+' || rn[0] == '-' || rn[0] == '*' || rn[0] == '/':
			return lexparse.StateFn(stateOper), nil
		case rn[0] == ' ':
			if _, dErr := l.Discard(1); dErr != nil {
				return nil, fmt.Errorf("lexing expression: %w", dErr)
			}
		default:
			return nil, fmt.Errorf("lexing expression: invalid character: %v", rn[0])
		}
	}
}

// stateNum parses a numerical value.
func stateNum(_ context.Context, l *lexparse.Lexer) (lexparse.State, error) {
	for {
		rn, err := l.Peek(1)
		if err != nil {
			return nil, fmt.Errorf("lexing number: %w", err)
		}
		// Not a digit character.
		if (rn[0] < 48 || rn[0] > 57) && rn[0] != '.' {
			// Emit lexeme.
			lexeme := l.Lexeme(numType)
			if lexeme.Value != "" {
				l.Emit(lexeme)
			}

			return lexparse.StateFn(stateExpr), nil
		}

		// Still lexing number.
		if _, dErr := l.Discard(1); dErr != nil {
			return nil, fmt.Errorf("lexing expression: %w", dErr)
		}
	}
}

// stateOpenParen parses an open parenthesis.
func stateOpenParen(_ context.Context, l *lexparse.Lexer) (lexparse.State, error) {
	_, err := l.Advance(1)
	if err != nil {
		return nil, fmt.Errorf("lexing open parenthesis: %w", err)
	}
	lexeme := l.Lexeme(openParenType)
	if lexeme.Value != "" {
		l.Emit(lexeme)
	}
	return lexparse.StateFn(stateExpr), nil
}

// stateCloseParen parses an close parenthesis.
func stateCloseParen(_ context.Context, l *lexparse.Lexer) (lexparse.State, error) {
	_, err := l.Advance(1)
	if err != nil {
		return nil, fmt.Errorf("lexing close parenthesis: %w", err)
	}
	lexeme := l.Lexeme(closeParenType)
	if lexeme.Value != "" {
		l.Emit(lexeme)
	}
	return lexparse.StateFn(stateExpr), nil
}

// stateOper parses an operator value.
func stateOper(_ context.Context, l *lexparse.Lexer) (lexparse.State, error) {
	_, err := l.Advance(1)
	if err != nil {
		return nil, fmt.Errorf("lexing close parenthesis: %w", err)
	}
	lexeme := l.Lexeme(operType)
	if lexeme.Value != "" {
		l.Emit(lexeme)
	}
	return lexparse.StateFn(stateExpr), nil
}

const (
	numNodeType int = iota
	parenNodeType
	operNodeType
)

type numVal struct {
	num float64
}

type operVal struct {
	oper rune
}

type nodeVal struct {
	typ int
	numVal
	operVal
}

// parseExpr delegates to another parse function based on lexeme type.
func parseExpr(ctx context.Context, p *lexparse.Parser[*nodeVal]) (lexparse.ParseFn[*nodeVal], error) {
	l := p.Peek()
	if l == nil {
		return nil, nil
	}
	switch l.Type {
	case numType:
		return parseNum, nil
	case openParenType:
		return parseOpenParen, nil
	case closeParenType:
		return parseCloseParen, nil
	case operType:
		return parseOper, nil
	default:
		return nil, fmt.Errorf("parsing expression: unknown lexeme type: %v", l.Type)
	}
}

func parseNum(ctx context.Context, p *lexparse.Parser[*nodeVal]) (lexparse.ParseFn[*nodeVal], error) {
	l := p.Next()
	v, err := strconv.ParseFloat(l.Value, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing number: %w", err)
	}

	// TODO: Throw error if the current node is not an operator, open parenthesis, or root node.

	// Push number as a child of the current node.
	p.Push(&nodeVal{
		numVal: numVal{
			num: v,
		},
	})
	return parseExpr, nil
}

func parseOpenParen(ctx context.Context, p *lexparse.Parser[*nodeVal]) (lexparse.ParseFn[*nodeVal], error) {
	// TODO: implement parseOpenParen

	// TODO: Throw an error if current node is a number.
	// TODO: Add new parenthesis node as child of current node.
	return nil, nil
}

func parseCloseParen(ctx context.Context, p *lexparse.Parser[*nodeVal]) (lexparse.ParseFn[*nodeVal], error) {
	// TODO: implement parseCloseParen

	// TODO: How to know if we have mismatched parenthesis?
	// TODO: Throw an error if current node is an operator or root node.
	return nil, nil
}

func parseOper(ctx context.Context, p *lexparse.Parser[*nodeVal]) (lexparse.ParseFn[*nodeVal], error) {
	// TODO: implement parseOper

	// TODO: Replace current node with new operator node.
	// TODO: Add previous current node as child of operator node.
	return nil, nil
}

func calc(t *lexparse.Tree[*nodeVal]) float64 {
	// TODO: Run calculator based on tree.
	return 0.0
}

// Example_calc implements a simple postfix calculator language.
func Example_calc() {
	r := runeio.NewReader(strings.NewReader("15.2 * (1.8 + 7.9) - 2.5 / 3.0"))
	t, err := lexparse.LexParse(context.Background(), r, lexparse.StateFn(stateExpr), parseExpr)
	if err != nil {
		panic(err)
	}

	fmt.Println(calc(t))
	// Output: 146.60666666666665
}
