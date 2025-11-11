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
	"fmt"
	"io"
)

// Node is the structure for a single node in the parse tree.
type Node[V comparable] struct {
	Parent   *Node[V]
	Children []*Node[V]
	Value    V

	// Start is the start position in the input where the value was found.
	Start Position
}

func (n *Node[V]) String() string {
	return fmtNode(n, nil)
}

func fmtNode[V comparable](node *Node[V], lastRank []bool) string {
	nodeStr := ""

	for i := range len(lastRank) - 1 {
		if lastRank[i] {
			nodeStr += "    "
		} else {
			nodeStr += "│   "
		}
	}

	if len(lastRank) > 0 {
		if lastRank[len(lastRank)-1] {
			nodeStr += "└── "
		} else {
			nodeStr += "├── "
		}
	}

	nodeStr += fmt.Sprintf("%v (%v)\n", node.Value, node.Start)
	for i, child := range node.Children {
		newLastRank := make([]bool, len(lastRank)+1)
		copy(newLastRank, lastRank)
		newLastRank[len(lastRank)] = i == len(node.Children)-1
		nodeStr += fmtNode(child, newLastRank)
	}

	return nodeStr
}

// ParseState is the state of the current parsing state machine. It defines the
// logic to process the current state and returns the next state.
type ParseState[V comparable] interface {
	// Run executes the logic at the current state, returning an error if one is
	// encountered. Implementations are expected to add new [Node] objects to
	// the AST using [Parser.Push] or [Parser.Node). As necessary, new parser
	// state should be pushed onto the stack as needed using [Parser.PushState].
	Run(ctx context.Context, p *Parser[V]) error
}

type parseFnState[V comparable] struct {
	f func(context.Context, *Parser[V]) error
}

// Run implements ParseState.Run.
func (s *parseFnState[V]) Run(ctx context.Context, p *Parser[V]) error {
	if s.f == nil {
		return nil
	}

	return s.f(ctx, p)
}

// ParseStateFn creates a State from the given Run function.
func ParseStateFn[V comparable](f func(context.Context, *Parser[V]) error) ParseState[V] {
	return &parseFnState[V]{f}
}

type stack[V comparable] []ParseState[V]

func (s *stack[V]) push(v ParseState[V]) {
	*s = append(*s, v)
}

func (s *stack[V]) pop() ParseState[V] {
	if len(*s) == 0 {
		return nil
	}

	v := (*s)[len(*s)-1]
	*s = (*s)[:len(*s)-1]

	return v
}

// TokenSource is an interface that defines a source of tokens for the parser.
type TokenSource interface {
	// NextToken returns the next token from the source. When tokens are
	// exhausted, it returns a Token with Type set to [TokenTypeEOF].
	NextToken(ctx context.Context) *Token
}

// NewParser creates a new Parser that reads from the tokens channel. The
// parser is initialized with a root node with an empty value.
func NewParser[V comparable](tokens TokenSource, startingState ParseState[V]) *Parser[V] {
	root := &Node[V]{
		Start: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
	}
	p := &Parser[V]{
		stateStack: &stack[V]{},
		tokens:     tokens,
	}
	p.root = root
	p.node = root

	p.PushState(startingState)

	return p
}

// Parser reads the tokens produced by a Lexer and builds a parse tree. It is
// implemented as a stack of states ([ParseState]) in which each state implements
// it's own processing.
//
// Parser maintains a current position in the parse tree which can be utilized
// by parser states.
type Parser[V comparable] struct {
	// tokens is a the source of tokens for the parser.
	tokens TokenSource

	// stateStack is a stack of expected future states of the parser.
	stateStack *stack[V]

	// root is the root node of the parse tree.
	root *Node[V]

	// node is the current node under processing.
	node *Node[V]

	// token is the current token in the stream.
	token *Token

	// next is the next token in the stream.
	next *Token
}

// Parse builds a parse tree by repeatedly pulling [ParseState] objects from
// the stack and running them, starting with the initial state. Parsing can be
// canceled by ctx.
//
// The caller can request that the parser stop by canceling ctx.
func (p *Parser[V]) Parse(ctx context.Context) (*Node[V], error) {
	for {
		state := p.stateStack.pop()
		if state == nil {
			break
		}

		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != nil {
				//nolint:wrapcheck // no additional error context for error.
				return p.root, err
			}

			return p.root, nil
		default:
		}

		var err error
		if err = state.Run(ctx, p); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			//nolint:wrapcheck // no additional error context for error.
			return p.root, err
		}
	}

	return p.root, nil
}

// PushState pushes a number of new expected future states onto the state stack
// in reverse order.
func (p *Parser[V]) PushState(states ...ParseState[V]) {
	for i := len(states) - 1; i >= 0; i-- {
		p.stateStack.push(states[i])
	}
}

// SetRoot sets the root of the parse tree to the given node. The current node
// is also set to the root node. This is useful for resetting the parser to a
// new root node.
func (p *Parser[V]) SetRoot(root *Node[V]) {
	p.root = root
	p.node = root
}

// Root returns the root of the parse tree.
func (p *Parser[V]) Root() *Node[V] {
	return p.root
}

// Peek returns the next token from the lexer without consuming it.
func (p *Parser[V]) Peek(ctx context.Context) *Token {
	if p.next != nil {
		return p.next
	}

	p.next = p.tokens.NextToken(ctx)

	return p.next
}

// Next returns the next token from the lexer. This is the new current token
// position.
func (p *Parser[V]) Next(ctx context.Context) *Token {
	l := p.Peek(ctx)
	p.next = nil
	p.token = l

	return p.token
}

// Pos returns the current node position in the tree.
func (p *Parser[V]) Pos() *Node[V] {
	return p.node
}

// Push creates a new node, adds it as a child to the current node, updates
// the current node to the new node, and returns the new node.
func (p *Parser[V]) Push(v V) *Node[V] {
	p.node = p.Node(v)
	return p.node
}

// Node creates a new node at the current token position and adds it as a
// child to the current node. The current node is not updated.
func (p *Parser[V]) Node(v V) *Node[V] {
	n := p.NewNode(v)
	p.node.Children = append(p.node.Children, n)
	n.Parent = p.node

	return n
}

// NewNode creates a new node at the current token position and returns it
// without adding it to the tree.
func (p *Parser[V]) NewNode(v V) *Node[V] {
	var start Position
	if p.token != nil {
		start = p.token.Start
	}

	return &Node[V]{
		Value: v,
		Start: start,
	}
}

// Climb updates the current node position to the current node's parent
// returning the previous current node. It is a no-op that returns the root
// node if called on the root node. Updates the end position of the parent node
// to the end position of the current node.
func (p *Parser[V]) Climb() *Node[V] {
	n := p.node
	if p.node.Parent != nil {
		p.node = p.node.Parent
	}

	return n
}

// Replace replaces the current node with a new node with the given value. The
// old node is removed from the tree and it's value is returned. Can be used to
// replace the root node.
//
//nolint:ireturn // returning the generic interface is needed to return the previous value.
func (p *Parser[V]) Replace(v V) V {
	node := p.NewNode(v)

	// Replace the parent.
	node.Parent = p.node.Parent
	if node.Parent != nil {
		for i := range node.Parent.Children {
			if node.Parent.Children[i] == p.node {
				node.Parent.Children[i] = node
				break
			}
		}
	}

	// Replace children. Preserve nil, non-nil slice.
	if p.node.Children != nil {
		node.Children = make([]*Node[V], len(p.node.Children))
		for i := range p.node.Children {
			node.Children[i] = p.node.Children[i]
			node.Children[i].Parent = node
		}
	}

	// If we are currently at the root, replace the root reference as well.
	if p.node == p.root {
		p.root = node
	}

	oldVal := p.node.Value
	p.node = node

	return oldVal
}
