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
	"io"
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
		case rn[0] == ' ' || rn[0] == '\t':
			if _, dErr := l.Discard(1); dErr != nil {
				return nil, fmt.Errorf("lexing expression: %w", dErr)
			}
		default:
			return nil, fmt.Errorf("unexpected character '%s' at position %d", string(rn[0]), l.Pos()+1)
		}
	}
}

// stateNum parses a numerical value.
func stateNum(_ context.Context, l *lexparse.Lexer) (lexparse.State, error) {
	for {
		rn, err := l.Peek(1)

		// Not a digit character.
		if errors.Is(err, io.EOF) || (err == nil && (rn[0] < 48 || rn[0] > 57) && rn[0] != '.') {
			// Emit lexeme.
			lexeme := l.Lexeme(numType)
			if lexeme.Value != "" {
				l.Emit(lexeme)
			}

			return lexparse.StateFn(stateExpr), nil
		}

		if err != nil {
			return nil, fmt.Errorf("lexing number: %w", err)
		}

		// Still lexing number.
		if _, dErr := l.Advance(1); dErr != nil {
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

func (v nodeVal) String() string {
	if v.typ == numNodeType {
		return fmt.Sprintf("%v", v.num)
	}
	if v.typ == parenNodeType {
		return "()"
	}
	if v.typ == operNodeType {
		return string(v.oper)
	}
	return ""
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
		return nil, fmt.Errorf("unexpected token '%s' at position %d", l.Value, l.Pos+1)
	}
}

func parseNum(ctx context.Context, p *lexparse.Parser[*nodeVal]) (lexparse.ParseFn[*nodeVal], error) {
	l := p.Next()

	// Throw error if the current node is not the root node, an operator, or open parenthesis.
	cur := p.Pos()
	if cur.Value != nil && cur.Value.typ != operNodeType && cur.Value.typ != parenNodeType {
		return nil, fmt.Errorf("unexpected number '%s' at position %d", l.Value, l.Pos+1)
	}

	v, err := strconv.ParseFloat(l.Value, 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing number '%s' at position %d: %w", l.Value, l.Pos+1, err)
	}

	// Push number as a child of the current node.
	p.Push(&nodeVal{
		typ: numNodeType,
		numVal: numVal{
			num: v,
		},
	})

	return parseExpr, nil
}

func parseOpenParen(ctx context.Context, p *lexparse.Parser[*nodeVal]) (lexparse.ParseFn[*nodeVal], error) {
	l := p.Next()

	// Throw error if the current node is not the root node, an operator, or open parenthesis.
	cur := p.Pos()
	if cur.Value != nil && cur.Value.typ != operNodeType && cur.Value.typ != parenNodeType {
		return nil, fmt.Errorf("unexpected parentheses '%s' at position %d", l.Value, l.Pos+1)
	}

	// Add new parenthesis node as child of current node.
	p.Push(&nodeVal{
		typ: parenNodeType,
	})

	return parseExpr, nil
}

func parseCloseParen(ctx context.Context, p *lexparse.Parser[*nodeVal]) (lexparse.ParseFn[*nodeVal], error) {
	l := p.Next()

	// Check for unexpected position.
	cur := p.Pos()
	if cur.Value == nil || cur.Value.typ != numNodeType {
		return nil, fmt.Errorf("unexpected parentheses '%s' at position %d", l.Value, l.Pos+1)
	}

	// Find the parenthesis node.
	for {
		cur := p.Climb()

		// If we find the root node then we have unopened parenthesis.
		if cur.Value == nil {
			return nil, fmt.Errorf("unexpected parentheses '%s' at position %d", l.Value, l.Pos+1)
		}

		if cur.Value.typ == parenNodeType {
			// We found the open parenthesis and reset the node position to its parent.
			break
		}
	}

	return parseExpr, nil
}

func parseOper(ctx context.Context, p *lexparse.Parser[*nodeVal]) (lexparse.ParseFn[*nodeVal], error) {
	l := p.Next()

	// Check for unexpected position.
	cur := p.Pos()

	// We are expected to not be at the root node.
	if cur.Value == nil {
		return nil, fmt.Errorf("unexpected operator '%s' at position %d", l.Value, l.Pos)
	}

	// If the current node is an incomplete operator then we have encountered
	// something like '1.5 * - 6'
	if cur.Value.typ == operNodeType && len(cur.Children) < 2 {
		return nil, fmt.Errorf("unexpected operator '%s' at position %d", l.Value, l.Pos+1)
	}

	// Add the new operator node and rotate the sub-tree such that the operator
	// node is the parent of the previous node.
	_ = p.SetRight(p.Node(&nodeVal{
		typ: operNodeType,
		operVal: operVal{
			oper: rune(l.Value[0]),
		},
	}))

	printNode(p.Pos(), 0)
	_ = p.RotateLeft()
	printNode(p.Pos(), 0)

	// TODO: Implement order of operations
	// Multiplication and division need to happen first. If the parent operators
	// are one of these, rotate again.
	// if oper == '*' || oper == '/' {
	//	rotateLeft(p)
	// }

	return parseExpr, nil
}

func rotateLeft(p *lexparse.Parser[*nodeVal]) {
	root := p.Pos()

	// If there is no right side of the current root then this is a
	// a no-op.
	if len(root.Children) < 2 {
		return
	}

	parent := root.Parent
	right := root.Children[1] // B

	if len(right.Children) > 0 {
		leftOfRight := right.Children[0]

		// Set parent of left child to the root node.
		leftOfRight.Parent = root

		// Set left child of the root's right node to the root.
		right.Children[0] = root

		// Set right child of the root node to the left child of the
		// right node.
		root.Children[1] = leftOfRight
	}

	// Set parent of the right child to the parent of the root
	right.Parent = parent
	// Set parent of root to the right child.
	root.Parent = right

	// Set the current node to the new root node.
	p.Climb()
}

func calc(t *lexparse.Node[*nodeVal]) float64 {
	// TODO: Run calculator based on tree.
	return 0.0
}

// TODO: Remove
func printNode[V comparable](n *lexparse.Node[V], depth int) {
	fmt.Printf("%v\n", n.Value)
	for _, c := range n.Children {
		for i := 0; i < depth+1; i++ {
			fmt.Print("\t")
		}
		printNode(c, depth+1)
	}
}

// Example_calc implements a simple postfix calculator language.
func Example_calc() {
	r := runeio.NewReader(strings.NewReader("15.2 * (1.8 + 7.9) - 2.5 / 3.0"))
	t, err := lexparse.LexParse(context.Background(), r, lexparse.StateFn(stateExpr), parseExpr)
	if err != nil {
		printNode(t, 0)
		return
		// panic(err)
	}

	// TODO: Remove
	// printNode(t, 0)

	fmt.Println(calc(t))
	// Output: 146.60666666666665
}
