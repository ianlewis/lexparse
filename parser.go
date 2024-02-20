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
)

// ErrMissingRequiredNode means the tree is missing nodes required to
// perform an operation.
var ErrMissingRequiredNode = errors.New("missing required node")

// Tree is the parse tree data structure.
type Tree[V comparable] struct {
	// Root points to the root Node in the parse tree.
	Root *Node[V]
}

// Node is the structure for a single node in the parse tree.
type Node[V comparable] struct {
	Parent   *Node[V]
	Children []*Node[V]
	Value    V

	// Pos is the position in the input where the value was found.
	Pos int

	// Line is the line number in the input where the value was found.
	Line int

	// Column is the column in the line of the input where the value was found.
	Column int
}

// ParseFn is the signature for the parsing function used to build the
// parse tree from lexemes. The parsing function is passed to
// Parse().
// There may be more than one parsing function used by a parser. The
// top-level function is passed to Parse(). A parsing function hands
// parsing off to another function by returning a pointer to the other
// function. Parse() will continue calling returned functions until
// nil is returned.
type ParseFn[V comparable] func(context.Context, *Parser[V]) (ParseFn[V], error)

// NewParser creates a new Parser that reads from the lexemes channel.
func NewParser[V comparable](lexemes <-chan *Lexeme) *Parser[V] {
	root := &Node[V]{}
	p := &Parser[V]{
		lexemes: lexemes,
		tree: &Tree[V]{
			Root: root,
		},
		node: root,
	}
	return p
}

// Parser reads the lexemes produced by a Lexer and builds a parse tree.
type Parser[V comparable] struct {
	lexemes <-chan *Lexeme

	tree *Tree[V]
	// node is the current node under processing.
	node *Node[V]

	// lexeme is the next lexeme in the stream.
	lexeme *Lexeme
}

// Parse builds a parse tree by repeatedly calling parseFn. parseFn
// takes cxt and the Parser as arguments and returns the parseFn and
// an error. The parse tree is built when parseFn returns nil for the
// parseFn. Parsing can be cancelled by ctx.
func (p *Parser[V]) Parse(ctx context.Context, parseFn ParseFn[V]) (*Tree[V], error) {
	for {
		if parseFn == nil {
			break
		}
		select {
		case <-ctx.Done():
			return p.Tree(), ctx.Err()
		default:
		}

		var err error
		parseFn, err = parseFn(ctx, p)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return p.Tree(), err
		}
	}
	return p.Tree(), nil
}

// Tree returns the parse Tree.
func (p *Parser[V]) Tree() *Tree[V] {
	return p.tree
}

// Peek returns the next Lexeme from the lexer without consuming it.
func (p *Parser[V]) Peek() *Lexeme {
	if p.lexeme != nil {
		return p.lexeme
	}
	l, ok := <-p.lexemes
	if !ok {
		return nil
	}
	p.lexeme = l
	return p.lexeme
}

// Next returns the next Lexeme from the lexer.
func (p *Parser[V]) Next() *Lexeme {
	l := p.Peek()
	p.lexeme = nil
	return l
}

// Pos returns the current node position in the tree.
func (p *Parser[V]) Pos() *Node[V] {
	return p.node
}

// Push creates a new node, adds it as a child to the current node, and sets it
// as the current node.
func (p *Parser[V]) Push(v V) *Node[V] {
	n := p.Node(v)
	p.node = n
	return n
}

// Node creates a new node and adds it as a child to the current node.
func (p *Parser[V]) Node(v V) *Node[V] {
	cur := p.Pos()
	var pos, line, col int
	if p.lexeme != nil {
		pos = p.lexeme.Pos
		line = p.lexeme.Line
		col = p.lexeme.Column
	}

	node := &Node[V]{
		Parent: p.Pos(),
		Value:  v,
		Pos:    pos,
		Line:   line,
		Column: col,
	}
	cur.Children = append(cur.Children, node)
	return node
}

// Pop updates the current node position to the current node's parent
// returning the previous current node.
func (p *Parser[V]) Pop() *Node[V] {
	n := p.node
	p.node = p.node.Parent
	return n
}

// RotateLeft restructures the tree by moving the current node to the
// position of its parent node. The original parent node becomes a
// child of the current node. All other child nodes of the original
// parent remain unchanged, except for the current node which becomes
// the new parent. If RotateLeft is called on the root node then
// ErrMissingRequiredNode is returned.
func (p *Parser[V]) RotateLeft() (*Node[V], error) {
	// n = node , op = original parent , gp = grand parent
	n := p.node

	op := n.Parent
	if op == nil {
		return nil, ErrMissingRequiredNode
	}

	gp := op.Parent

	// Remove n from op's Children
	opChildren := op.Children[:0]
	for _, x := range op.Children {
		if x != n {
			opChildren = append(opChildren, x)
		}
	}
	op.Children = opChildren

	// Add op to n's Children
	n.Children = append(n.Children, op)

	// Update op's Parent
	op.Parent = n

	// Update n's Parent
	n.Parent = gp

	// Replace op with n in gp's Children
	if gp != nil {
		gpChildren := gp.Children[:0]
		for _, x := range gp.Children {
			if x != op {
				gpChildren = append(gpChildren, x)
			} else {
				gpChildren = append(gpChildren, n)
			}
		}
		gp.Children = gpChildren
	}

	// Update the tree root if needed
	if p.Tree().Root == op {
		p.Tree().Root = n
	}

	return n, nil
}

// AdoptSibling moves the current node's previous sibling into the node's
// child. If no previous sibling exists, ErrMissingRequiredNode is returned.
func (p *Parser[V]) AdoptSibling() (*Node[V], error) {
	// n = node , op = original parent , s = sibling
	n := p.node
	op := n.Parent
	var s *Node[V]

	if op == nil {
		return nil, ErrMissingRequiredNode
	}

	// Find the previous sibling
	for _, x := range op.Children {
		if x == n {
			break
		}
		s = x
	}

	if s == nil {
		return nil, ErrMissingRequiredNode
	}

	// Remove s from op's Children
	opChildren := op.Children[:0]
	for _, x := range op.Children {
		if x != s {
			opChildren = append(opChildren, x)
		}
	}
	op.Children = opChildren

	// Add s to n's Children
	n.Children = append(n.Children, s)

	// Update s's Parent
	s.Parent = n

	return n, nil
}
