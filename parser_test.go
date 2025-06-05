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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ianlewis/runeio"
)

func newTree[V comparable](n ...*Node[V]) *Node[V] {
	root := &Node[V]{}
	root.Children = append(root.Children, n...)
	return addParent(root)
}

// addParent sets the parent reference on all children of n.
func addParent[V comparable](n *Node[V]) *Node[V] {
	if n != nil {
		for _, c := range n.Children {
			c.Parent = n
			_ = addParent(c)
		}
	}
	return n
}

// testLexer creates and returns a lexer.
func testLexer(t *testing.T, input string) <-chan *Token {
	t.Helper()

	// Run the lexer filling the channel buffer. Test input should not exceed this buffer.
	tokens := make(chan *Token, 1024)
	l := NewLexer(runeio.NewReader(strings.NewReader(input)), tokens, &lexWordState{})

	if err := l.Lex(context.Background()); err != nil {
		panic(err)
	}

	return tokens
}

// testParse creates and runs a lexer, and returns the root of the parse tree.
func testParse(t *testing.T, input string) (*Node[string], error) {
	t.Helper()

	tokens := testLexer(t, input)

	p := NewParser[string](tokens, ParseStateFn(func(_ context.Context, p *Parser[string]) error {
		for {
			token := p.Next()
			switch token.Type {
			case wordType:
				// OK
			case TokenTypeEOF:
				return nil
			default:
				panic("unknown type")
			}

			switch token.Value {
			case "climb":
				// Climb the tree without adding a node.
				_ = p.Climb()
			case "replace":
				_ = p.Replace(token.Value)
			case "push":
				_ = p.Push(token.Value)
			default:
				p.Node(token.Value)
			}
		}
	}))

	root, err := p.Parse(context.Background())
	return root, err
}

func TestParser_new(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil, nil)

	expectedRoot := &Node[string]{}
	if diff := cmp.Diff(expectedRoot, p.root); diff != "" {
		t.Fatalf("NewParser: p.root (-want, +got): \n%s", diff)
	}

	if diff := cmp.Diff(expectedRoot, p.node); diff != "" {
		t.Errorf("NewParser: p.node (-want, +got): \n%s", diff)
	}
}

// TestParser_parse_op2 builds a tree of 2-child operations.
func TestParser_parse_op2(t *testing.T) {
	t.Parallel()

	input := "push 1 push 2 3"

	root, err := testParse(t, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Does the tree look as expected?
	expectedRoot := newTree(&Node[string]{
		Value:  "push",
		Line:   1,
		Column: 1,
		Pos:    0,
		Children: []*Node[string]{
			{
				Value:  "1",
				Line:   1,
				Column: 6,
				Pos:    5,
			},
			{
				Value:  "push",
				Line:   1,
				Column: 8,
				Pos:    7,
				Children: []*Node[string]{
					{
						Value:  "2",
						Line:   1,
						Column: 13,
						Pos:    12,
					},
					{
						Value:  "3",
						Line:   1,
						Column: 15,
						Pos:    14,
					},
				},
			},
		},
	})

	if diff := cmp.Diff(expectedRoot, root); diff != "" {
		t.Fatalf("Parse: root (-want, +got): \n%s", diff)
	}
}

func TestParser_NextPeek(t *testing.T) {
	t.Parallel()

	input := "A B C"
	tokens := testLexer(t, input)

	p := NewParser[string](tokens, nil)

	// expect to read the first token "A"
	tokenA := p.Next()
	wanttokenA := &Token{
		Type:   wordType,
		Value:  "A",
		Pos:    0,
		Line:   1,
		Column: 1,
	}
	if diff := cmp.Diff(wanttokenA, tokenA); diff != "" {
		t.Fatalf("Next: (-want, +got): \n%s", diff)
	}

	peekTokenB := p.Peek()
	wantTokenB := &Token{
		Type:   wordType,
		Value:  "B",
		Pos:    2,
		Line:   1,
		Column: 3,
	}
	if diff := cmp.Diff(wantTokenB, peekTokenB); diff != "" {
		t.Fatalf("Peek: (-want, +got): \n%s", diff)
	}

	// expect to read the second token "B" because it was not consumed
	tokenB := p.Next()
	if diff := cmp.Diff(wantTokenB, tokenB); diff != "" {
		t.Fatalf("Peek: (-want, +got): \n%s", diff)
	}

	tokenC := p.Next()
	wantTokenC := &Token{
		Type:   wordType,
		Value:  "C",
		Pos:    4,
		Line:   1,
		Column: 5,
	}
	if diff := cmp.Diff(wantTokenC, tokenC); diff != "" {
		t.Fatalf("Next: (-want, +got): \n%s", diff)
	}

	// expected end of tokens
	niltoken := p.Next()
	tokenEOF := &Token{
		Type:   TokenTypeEOF,
		Value:  "",
		Pos:    5,
		Line:   1,
		Column: 6,
	}
	if diff := cmp.Diff(tokenEOF, niltoken); diff != "" {
		t.Fatalf("Next: (-want, +got): \n%s", diff)
	}
}

func TestParser_Node(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil, nil)

	child1 := p.Node("A")
	expectedRootA := newTree(
		&Node[string]{
			Value: "A",
		},
	)

	if diff := cmp.Diff(expectedRootA.Children[0], child1); diff != "" {
		t.Fatalf("Node: (-want, +got): \n%s", diff)
	}
	// Current node is still set to root.
	if diff := cmp.Diff(p.root, p.node); diff != "" {
		t.Errorf("p.node: (-want, +got): \n%s", diff)
	}

	child2 := p.Node("B")
	expectedRootB := newTree(
		&Node[string]{
			Value: "A",
		},
		&Node[string]{
			Value: "B",
		},
	)

	if diff := cmp.Diff(expectedRootB.Children[1], child2); diff != "" {
		t.Fatalf("Node: (-want, +got): \n%s", diff)
	}
	// Current node is still set to root.
	if diff := cmp.Diff(p.root, p.node); diff != "" {
		t.Errorf("p.node: (-want, +got): \n%s", diff)
	}

	if diff := cmp.Diff(expectedRootB, p.root); diff != "" {
		t.Fatalf("Node: p.root (-want, +got): \n%s", diff)
	}
}

func TestParser_ClimbPos(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil, nil)

	p.root = newTree(
		&Node[string]{
			Value: "A",
			Children: []*Node[string]{
				{
					Value: "B",
				},
			},
		},
	)
	// Current node is Node B
	p.node = p.root.Children[0].Children[0]

	// Climb returns Node B
	if diff := cmp.Diff(p.root.Children[0].Children[0], p.Climb()); diff != "" {
		t.Errorf("Climb: (-want, +got): \n%s", diff)
	}
	// Current node is set to Node A
	if diff := cmp.Diff(p.root.Children[0], p.node); diff != "" {
		t.Errorf("p.node: (-want, +got): \n%s", diff)
	}
	// Pos returns Node A
	if diff := cmp.Diff(p.root.Children[0], p.Pos()); diff != "" {
		t.Errorf("Pos: (-want, +got): \n%s", diff)
	}

	// Climb returns Node A
	if diff := cmp.Diff(p.root.Children[0], p.Climb()); diff != "" {
		t.Errorf("Climb: (-want, +got): \n%s", diff)
	}
	// Current node is set to root node.
	if diff := cmp.Diff(p.root, p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	// Pos returns root node.
	if diff := cmp.Diff(p.root, p.Pos()); diff != "" {
		t.Errorf("Pos: (-want, +got): \n%s", diff)
	}

	// Climb returns root node.
	if diff := cmp.Diff(p.root, p.Climb()); diff != "" {
		t.Errorf("Climb: (-want, +got): \n%s", diff)
	}
	// Current node is set to root node.
	if diff := cmp.Diff(p.root, p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	// Pos returns root node.
	if diff := cmp.Diff(p.root, p.Pos()); diff != "" {
		t.Errorf("Pos: (-want, +got): \n%s", diff)
	}
}

func TestParser_Push(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil, nil)

	valA := "A"
	expectedRootA := newTree(
		&Node[string]{
			Value: valA,
		},
	)
	if diff := cmp.Diff(expectedRootA.Children[0], p.Push(valA)); diff != "" {
		t.Errorf("Push(%q): (-want, +got): \n%s", valA, diff)
	}
	if diff := cmp.Diff(expectedRootA.Children[0], p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	if diff := cmp.Diff(expectedRootA, p.root); diff != "" {
		t.Errorf("p.root (-want, +got): \n%s", diff)
	}

	valB := "B"
	expectedRootB := newTree(
		&Node[string]{
			Value: "A",
			Children: []*Node[string]{
				{
					Value: "B",
				},
			},
		},
	)
	if diff := cmp.Diff(expectedRootB.Children[0].Children[0], p.Push(valB)); diff != "" {
		t.Errorf("Push(%q): (-want, +got): \n%s", valB, diff)
	}
	if diff := cmp.Diff(expectedRootB.Children[0].Children[0], p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	if diff := cmp.Diff(expectedRootB, p.root); diff != "" {
		t.Errorf("p.root (-want, +got): \n%s", diff)
	}
}

func TestParser_Replace(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil, nil)

	p.root = newTree(
		&Node[string]{
			Value: "A",
			Children: []*Node[string]{
				{
					Value: "B",
				},
			},
		},
	)
	// Current node is Node A
	p.node = p.root.Children[0]

	// Replace Node A with C
	valC := "C"
	if diff := cmp.Diff("A", p.Replace(valC)); diff != "" {
		t.Errorf("Replace(%q): (-want, +got): \n%s", valC, diff)
	}

	expectedRoot := newTree(
		&Node[string]{
			Value: "C",
			Children: []*Node[string]{
				{
					Value: "B",
				},
			},
		},
	)
	// Current node is set to Node C.
	if diff := cmp.Diff(expectedRoot.Children[0], p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	// Full tree has expected values.
	if diff := cmp.Diff(expectedRoot, p.root); diff != "" {
		t.Errorf("p.root (-want, +got): \n%s", diff)
	}
}

func TestParser_Replace_root(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil, nil)

	// Replace root node with A
	valA := "A"
	if diff := cmp.Diff("", p.Replace(valA)); diff != "" {
		t.Errorf("Replace(%q): (-want, +got): \n%s", valA, diff)
	}

	expectedRoot := &Node[string]{
		Value: "A",
	}

	// Current node is set to root node.
	if diff := cmp.Diff(expectedRoot, p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	// Full tree has expected values.
	if diff := cmp.Diff(expectedRoot, p.root); diff != "" {
		t.Errorf("p.root (-want, +got): \n%s", diff)
	}
}
