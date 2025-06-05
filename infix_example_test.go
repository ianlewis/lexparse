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
	errUnclosedParen        = errors.New("unclosed parenthesis")
	errUnexpectedParen      = errors.New("unexpected closing parenthesis")
	errDivByZero            = errors.New("division by zero")

	errInvalidNode = errors.New("invalid node")
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

func (n *exprNode) precedence() int {
	if n.typ != nodeTypeOper {
		panic(fmt.Sprintf("node %v is not an operator node", n))
	}
	switch n.oper {
	case "+", "-":
		return 1
	case "*", "/":
		return 2
	default:
		return 0
	}
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

// pratt implements a Pratt operator-precedence parser for infix expressions.
func pratt(ctx context.Context, parser *lexparse.Parser[*exprNode]) error {
	n, err := parseExpr(ctx, parser, 0, 0)
	parser.SetRoot(n)
	return err
}

func parseExpr(ctx context.Context, parser *lexparse.Parser[*exprNode], depth int, minPrecedence int) (*lexparse.Node[*exprNode], error) {
	t := parser.Next()
	var lhs *lexparse.Node[*exprNode]
	switch t.Type {
	case lexTypeNum:
		num, err := strconv.ParseFloat(t.Value, 64)
		if err != nil {
			return nil, tokenErr(err, t)
		}
		lhs = parser.NewNode(&exprNode{
			typ: nodeTypeNum,
			num: num,
		})
	case lexTypeOpenParen:
		// Parse the expression inside the parentheses.
		lhs2, err := parseExpr(ctx, parser, depth+1, 0)
		if err != nil {
			return nil, err
		}
		lhs = lhs2
		t2 := parser.Next()
		if t2.Type != lexTypeCloseParen {
			return nil, tokenErr(errUnclosedParen, t2)
		}
	case lexparse.TokenTypeEOF:
		return nil, tokenErr(io.ErrUnexpectedEOF, t)
	default:
		return nil, tokenErr(errUnexpectedIdentifier, t)
	}

outerL:
	for {
		var opVal *exprNode
		opToken := parser.Peek()
		switch opToken.Type {
		case lexTypeOper:
			opVal = &exprNode{
				typ:  nodeTypeOper,
				oper: opToken.Value,
			}
		case lexparse.TokenTypeEOF:
			break outerL
		case lexTypeCloseParen:
			if depth == 0 {
				return nil, tokenErr(errUnexpectedParen, opToken)
			}
			break outerL
		default:
			return nil, tokenErr(errUnexpectedIdentifier, opToken)
		}

		if opVal.precedence() < minPrecedence {
			// If the operator precedence is less than the minimum precedence,
			// stop parsing.
			return lhs, nil
		}

		_ = parser.Next() // Consume the operator token.
		opNode := parser.NewNode(opVal)

		rhs, err := parseExpr(ctx, parser, depth, opNode.Value.precedence())
		if err != nil {
			return nil, err
		}

		// Add the operator's children.
		opNode.Children = append(opNode.Children, lhs, rhs)
		lhs = opNode
	}

	return lhs, nil
}

type parseRHS struct {
	lhs *exprNode
}

// parseRHS adds the parsed right-hand side of an expression with the left-hand
// side to the AST.
func (p *parseRHS) Run(ctx context.Context, parser *lexparse.Parser[*exprNode]) error {
	return nil
}

// Calculate performs calculation based on the parsed expression tree.
func Calculate(root *lexparse.Node[*exprNode]) (float64, error) {
	switch root.Value.typ {
	case nodeTypeNum:
		return root.Value.num, nil
	case nodeTypeOper:
		if len(root.Children) != 2 {
			return 0.0, fmt.Errorf("%w: invalid children: %v", errInvalidNode, root.Value)
		}
		left, err := Calculate(root.Children[0])
		if err != nil {
			return 0.0, err
		}
		right, err := Calculate(root.Children[1])
		if err != nil {
			return 0.0, err
		}
		switch root.Value.oper {
		case "+":
			return left + right, nil
		case "-":
			return left - right, nil
		case "*":
			return left * right, nil
		case "/":
			if right == 0 {
				return 0.0, errDivByZero
			}
			return left / right, nil
		default:
			return 0.0, fmt.Errorf("%w: operator: %s", errInvalidNode, root.Value.oper)
		}
	default:
		return 0.0, fmt.Errorf("%w: node type: %v", errInvalidNode, root.Value.typ)
	}
}

func Example_infixCalculator() {
	tokens := make(chan *lexparse.Token, 1024)
	r := runeio.NewReader(strings.NewReader(`6.1 * ( 2.8 + 3.2 ) / 7.6 - 2.4`))

	t, err := lexparse.LexParse(
		context.Background(),
		lexparse.NewLexer(r, tokens, lexparse.LexStateFn(lexExpression)),
		lexparse.NewParser(tokens, lexparse.ParseStateFn(pratt)),
	)
	if err != nil {
		panic(err)
	}
	txt, err := Calculate(t)
	if err != nil {
		panic(err)
	}
	fmt.Print(txt)

	// Output: 2.4157894736842107
}
