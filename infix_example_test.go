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

	"github.com/ianlewis/lexparse"
)

var (
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

func (n *exprNode) String() string {
	switch n.typ {
	case nodeTypeNum:
		return fmt.Sprintf("%g", n.num)
	case nodeTypeOper:
		return n.oper
	default:
		return fmt.Sprintf("UnknownNodeType(%d)", n.typ)
	}
}

func tokenErr(err error, t *lexparse.Token) error {
	return fmt.Errorf("%w: %q, line %d, column %d", err,
		t.Value, t.Start.Line, t.Start.Column)
}

// pratt implements a Pratt operator-precedence parser for infix expressions.
func pratt(ctx context.Context, cur *lexparse.ParseCursor[*exprNode]) error {
	n, err := parseExpr(ctx, cur, 0, 0)
	cur.SetRoot(n)

	return err
}

func parseExpr(
	ctx context.Context,
	cur *lexparse.ParseCursor[*exprNode],
	depth, minPrecedence int,
) (*lexparse.Node[*exprNode], error) {
	// Check if the context is canceled.
	select {
	case <-ctx.Done():
		//nolint:wrapcheck // We want to return the original context error.
		return nil, ctx.Err()
	default:
	}

	token := cur.Next(ctx)

	var lhs *lexparse.Node[*exprNode]

	switch token.Type {
	case lexparse.TokenTypeFloat, lexparse.TokenTypeInt:
		num, err := strconv.ParseFloat(token.Value, 64)
		if err != nil {
			return nil, tokenErr(err, token)
		}

		lhs = cur.NewNode(&exprNode{
			typ: nodeTypeNum,
			num: num,
		})
	case '(':
		// Parse the expression inside the parentheses.
		lhs2, err := parseExpr(ctx, cur, depth+1, 0)
		if err != nil {
			return nil, err
		}

		lhs = lhs2

		t2 := cur.Next(ctx)
		if t2.Type != ')' {
			return nil, tokenErr(errUnclosedParen, t2)
		}
	case lexparse.TokenTypeEOF:
		return nil, tokenErr(io.ErrUnexpectedEOF, token)
	default:
		return nil, tokenErr(errUnexpectedIdentifier, token)
	}

outerL:
	for {
		var opVal *exprNode

		opToken := cur.Peek(ctx)
		switch opToken.Type {
		case '+', '-', '*', '/':
			opVal = &exprNode{
				typ:  nodeTypeOper,
				oper: opToken.Value,
			}
		case lexparse.TokenTypeEOF:
			break outerL
		case ')':
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

		_ = cur.Next(ctx) // Consume the operator token.
		opNode := cur.NewNode(opVal)

		rhs, err := parseExpr(ctx, cur, depth, opNode.Value.precedence())
		if err != nil {
			return nil, err
		}

		// Add the operator's children.
		opNode.Children = append(opNode.Children, lhs, rhs)
		lhs = opNode
	}

	return lhs, nil
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

// Example_infixCalculator demonstrates an infix expression calculator
// using a Pratt parser. It makes use of the ScanningLexer to tokenize
// the input expression and builds an expression tree that is then evaluated
// using the Calculate function.
func Example_infixCalculator() {
	r := strings.NewReader(`6.1 * ( 2.8 + 3.2 ) / 7.6 - 2.4`)

	tree, err := lexparse.LexParse(
		context.Background(),
		lexparse.NewScanningLexer(r),
		lexparse.ParseStateFn(pratt),
	)
	if err != nil {
		panic(err)
	}

	// Print the expression tree.
	fmt.Println(tree)

	txt, err := Calculate(tree)
	if err != nil {
		panic(err)
	}

	// Print the evaluation result.
	fmt.Print(txt)

	// Output:
	// - (1:27)
	// ├── * (1:5)
	// │   ├── 6.1 (1:1)
	// │   └── / (1:21)
	// │       ├── + (1:13)
	// │       │   ├── 2.8 (1:9)
	// │       │   └── 3.2 (1:15)
	// │       └── 7.6 (1:23)
	// └── 2.4 (1:29)
	//
	// 2.4157894736842107
}
