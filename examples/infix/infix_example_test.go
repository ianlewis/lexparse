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

package lexparse_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/ianlewis/runeio"

	"github.com/ianlewis/lexparse"
)

const (
	lexTypeNum lexparse.TokenType = iota
	lexTypeOpenParen
	lexTypeCloseParen
	lexTypeOper
)

var (
	errUnexpectedRune       = errors.New("unexpected rune")
	errUnexpectedIdentifier = errors.New("unexpected identifier")
)

type nodeType int

const (
	// nodeTypeNum is a leaf node comprised of a number.
	nodeTypeNum nodeType = iota

	// nodeTypeOper is a binary node whose children are the left and right.
	nodeTypeOper
)

// exprNode is a node in the expression tree.
type exprNode struct {
	typ  nodeType
	num  float64 // Only used for nodeTypeNum.
	oper string  // Only used for nodeTypeOper.
}

func tokenErr(err error, t *lexparse.Token) error {
	return fmt.Errorf("%w: %q, line %d, column %d", err, t.Value, t.Line, t.Column)
}

// lexExpression tokenizes normal text.
func lexExpression(_ context.Context, l *lexparse.Lexer) (lexparse.LexState, error) {
	for {
		rn := l.Peek()
		switch rn {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			return lexparse.LexStateFn(lexNum), nil
		case '(':
			// Open parenthesis.
			if !l.Advance() {
				panic(fmt.Errorf("%w: parsing expression", io.ErrUnexpectedEOF))
			}
			l.Emit(lexTypeOpenParen)
		case ')':
			// Close parenthesis.
			if !l.Advance() {
				panic(fmt.Errorf("%w: parsing expression", io.ErrUnexpectedEOF))
			}
			l.Emit(lexTypeCloseParen)
		case '+', '-', '*', '/':
			// Operator.
			if !l.Advance() {
				panic(fmt.Errorf("%w: parsing expression", io.ErrUnexpectedEOF))
			}
			l.Emit(lexTypeOper)
		case ' ', '\t':
			// Whitespace characters.
			if !l.Discard() {
				panic(fmt.Errorf("%w: parsing expression", io.ErrUnexpectedEOF))
			}
			continue
		case lexparse.EOF:
			// End of file.
			return nil, nil
		default:
			return nil, fmt.Errorf("%w: '%s'", errUnexpectedRune, string(rn))
		}
	}
}

// lexNum lexes a number from the input stream.
func lexNum(_ context.Context, l *lexparse.Lexer) (lexparse.LexState, error) {
	for {
		rn := l.Peek()
		switch rn {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
			// Digit character.
		case '.':
			// Decimal point.
		default:
			if l.Width() > 0 {
				l.Emit(lexTypeNum)
			}
			return lexparse.LexStateFn(lexExpression), nil
		}

		// Advance the input stream.
		if !l.Advance() {
			if l.Width() > 0 {
				l.Emit(lexTypeNum)
			}
			return nil, nil
		}
	}
}

// parseRoot is the same as [parseExpr] but it does not expect EOF.
func parseRoot(_ context.Context, p *lexparse.Parser[*exprNode]) error {
	t := p.Peek()
	if t.Type == lexparse.TokenTypeEOF {
		return tokenErr(io.ErrUnexpectedEOF, t)
	}
	p.PushState(lexparse.ParseStateFn(parseExpr))
	return nil
}

// parseExpr parses an expression from the input stream. It expects to encounter
// a number, open parenthesis, or EOF.
func parseExpr(_ context.Context, p *lexparse.Parser[*exprNode]) error {
	t := p.Next()
	fmt.Printf("parseExpr: %#v\n", t)
	switch t.Type {
	case lexTypeNum:
		// Create a leaf node for the number.
		num, err := strconv.ParseFloat(t.Value, 64)
		if err != nil {
			return tokenErr(err, t)
		}

		p.Push(&exprNode{
			typ: nodeTypeNum,
			num: num,
		})
		p.PushState(lexparse.ParseStateFn(parseOper))
	case lexTypeOpenParen:
		// Push the state to parse the expression inside the parenthesis.
		p.PushState(
			lexparse.ParseStateFn(parseExpr),
			lexparse.ParseStateFn(parseCloseParen),
		)
	case lexparse.TokenTypeEOF:
		return nil
	default:
		fmt.Printf("parseExpr: unexpected token: %#v\n", t)
		// return tokenErr(errUnexpectedIdentifier, t)
		return tokenErr(fmt.Errorf("parseExpr:%w", errUnexpectedIdentifier), t)
	}
	return nil
}

// parseOper parses an operator from the input stream. It expects to encounter
// an operator, close parenthesis, or EOF.
func parseOper(_ context.Context, p *lexparse.Parser[*exprNode]) error {
	t := p.Peek()
	fmt.Printf("parseOper: %#v\n", t)
	switch t.Type {
	case lexTypeOper:
		_ = p.Next() // Consume the operator token.

		// Create a node for the operator.
		p.Push(&exprNode{
			typ:  nodeTypeOper,
			oper: t.Value,
		})
		p.PushState(lexparse.ParseStateFn(parseExpr))
	case lexTypeCloseParen:
		// If we encounter a close parenthesis return so it can be handled.
		// parseCloseParen should already be on the stack.
	case lexparse.TokenTypeEOF:
		// return tokenErr(fmt.Errorf("%w: unclosed parenthesis?", io.ErrUnexpectedEOF), t)
	default:
		// return tokenErr(errUnexpectedIdentifier, t)
		return tokenErr(fmt.Errorf("parseOper:%w", errUnexpectedIdentifier), t)
	}
	return nil
}

func parseCloseParen(_ context.Context, p *lexparse.Parser[*exprNode]) error {
	t := p.Next()
	fmt.Printf("parseCloseParen: %#v\n", t)
	switch t.Type {
	case lexTypeCloseParen:
		// FIXME: Handle the close parenthesis.
		p.PushState(lexparse.ParseStateFn(parseOper))
	case lexparse.TokenTypeEOF:
		return fmt.Errorf("%w: unclosed parentheses", io.ErrUnexpectedEOF)
	default:
		// return tokenErr(errUnexpectedIdentifier, t)
		return tokenErr(fmt.Errorf("parseCloseParen:%w", errUnexpectedIdentifier), t)
	}
	return nil
}

// Execute performs calculation based on the parsed expression tree.
func Execute(root *lexparse.Node[*exprNode]) (float64, error) {
	return 0.0, nil
}

func Example_infixCalculator() {
	tokens := make(chan *lexparse.Token, 1024)
	r := runeio.NewReader(strings.NewReader(`6.1 * ( 2.8 + 3.2 ) / 7.6 - 2.4`))

	t, err := lexparse.LexParse(
		context.Background(),
		lexparse.NewLexer(r, tokens, lexparse.LexStateFn(lexExpression)),
		lexparse.NewParser(tokens, lexparse.ParseStateFn(parseRoot)),
	)
	if err != nil {
		panic(err)
	}
	txt, err := Execute(t)
	if err != nil {
		panic(err)
	}
	fmt.Print(txt)

	// Output: 2.2857142857142856
}
