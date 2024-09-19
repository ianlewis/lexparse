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
func testLexer(t *testing.T, input string) (<-chan *Lexeme, context.CancelFunc) {
	t.Helper()

	l := NewLexer(runeio.NewReader(strings.NewReader(input)), &wordState{})

	ctx, cancel := context.WithCancel(context.Background())
	return l.Lex(ctx), cancel
}

// testParse creates and runs a lexer, and returns the root of the parse tree.
func testParse(t *testing.T, input string) (*Node[string], error) {
	t.Helper()

	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)
	pFn := func(_ context.Context, p *Parser[string]) (ParseFn[string], error) {
		for {
			lexeme := p.Next()
			if lexeme == nil {
				break
			}

			switch lexeme.Value {
			case "climb":
				// Climb the tree without adding a node.
				_ = p.Climb()
			case "replace":
				_ = p.Replace(lexeme.Value)
			case "push":
				_ = p.Push(lexeme.Value)
			default:
				p.Node(lexeme.Value)
			}
		}
		return nil, nil
	}

	ctx := context.Background()
	root, err := p.Parse(ctx, pFn)
	return root, err
}

func TestParser_new(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

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
		Value: "push",
		Children: []*Node[string]{
			{
				Value: "1",
			},
			{
				Value: "push",
				Children: []*Node[string]{
					{
						Value: "2",
					},
					{
						Value: "3",
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
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	// expect to read the first lexeme "A"
	lexemeA := p.Next()
	wantLexemeA := &Lexeme{
		Type:   wordType,
		Value:  "A",
		Pos:    0,
		Line:   0,
		Column: 0,
	}
	if diff := cmp.Diff(wantLexemeA, lexemeA); diff != "" {
		t.Fatalf("Next: (-want, +got): \n%s", diff)
	}

	peekLexemeB := p.Peek()
	wantLexemeB := &Lexeme{
		Type:   wordType,
		Value:  "B",
		Pos:    2,
		Line:   0,
		Column: 2,
	}
	if diff := cmp.Diff(wantLexemeB, peekLexemeB); diff != "" {
		t.Fatalf("Peek: (-want, +got): \n%s", diff)
	}

	// expect to read the second lexeme "B" because it was not consumed
	lexemeB := p.Next()
	if diff := cmp.Diff(wantLexemeB, lexemeB); diff != "" {
		t.Fatalf("Peek: (-want, +got): \n%s", diff)
	}

	lexemeC := p.Next()
	wantLexemeC := &Lexeme{
		Type:   wordType,
		Value:  "C",
		Pos:    4,
		Line:   0,
		Column: 4,
	}
	if diff := cmp.Diff(wantLexemeC, lexemeC); diff != "" {
		t.Fatalf("Next: (-want, +got): \n%s", diff)
	}

	// expected end of lexemes
	nilLexeme := p.Next()
	if diff := cmp.Diff((*Lexeme)(nil), nilLexeme); diff != "" {
		t.Fatalf("Next: (-want, +got): \n%s", diff)
	}
}

func TestParser_Node(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

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

	p := NewParser[string](nil)

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

	p := NewParser[string](nil)

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

	p := NewParser[string](nil)

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

	p := NewParser[string](nil)

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

func TestParser_RotateLeft(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

	p.root = newTree(
		&Node[string]{
			Value: "P",
			Children: []*Node[string]{
				{
					Value: "A",
				},
				{
					Value: "Q",
					Children: []*Node[string]{
						{
							Value: "B",
						},
						{
							Value: "C",
						},
					},
				},
			},
		},
	)

	// Current node is Node P
	p.node = p.root.Children[0]

	newSubRoot := p.RotateLeft()

	// Expect that Q is rotated above P.
	expectedRoot := newTree(
		&Node[string]{
			Value: "Q",
			Children: []*Node[string]{
				{
					Value: "P",
					Children: []*Node[string]{
						{
							Value: "A",
						},
						{
							Value: "B",
						},
					},
				},
				{
					Value: "C",
				},
			},
		},
	)
	expectedSubRoot := expectedRoot.Children[0]

	// The new parent Node Q is returned.
	if diff := cmp.Diff(expectedSubRoot, newSubRoot); diff != "" {
		t.Fatalf("RotateLeft: (-want, +got): \n%s", diff)
	}
	// Current node is set to Q
	if diff := cmp.Diff(expectedSubRoot, p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	// Full tree has expected values.
	if diff := cmp.Diff(expectedRoot, p.root); diff != "" {
		t.Errorf("p.root (-want, +got): \n%s", diff)
	}
}

func TestParser_RotateLeft_root(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

	p.root = addParent(
		&Node[string]{
			Value: "P",
			Children: []*Node[string]{
				{
					Value: "A",
				},
				{
					Value: "Q",
					Children: []*Node[string]{
						{
							Value: "B",
						},
						{
							Value: "C",
						},
					},
				},
			},
		},
	)
	// Current node is Node P
	p.node = p.root

	newSubRoot := p.RotateLeft()

	// Expect that Q is rotated above P.
	expectedSubRoot := addParent(
		&Node[string]{
			Value: "Q",
			Children: []*Node[string]{
				{
					Value: "P",
					Children: []*Node[string]{
						{
							Value: "A",
						},
						{
							Value: "B",
						},
					},
				},
				{
					Value: "C",
				},
			},
		},
	)

	// The new parent Node Q is returned.
	if diff := cmp.Diff(expectedSubRoot, newSubRoot); diff != "" {
		t.Fatalf("RotateLeft: (-want, +got): \n%s", diff)
	}
	// Current node is set to Q
	if diff := cmp.Diff(expectedSubRoot, p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	// Full tree has expected values.
	if diff := cmp.Diff(expectedSubRoot, p.root); diff != "" {
		t.Errorf("p.root (-want, +got): \n%s", diff)
	}
}

func TestParser_RotateLeft_empty(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

	n := p.RotateLeft()
	if diff := cmp.Diff(p.node, n); diff != "" {
		t.Fatalf("RotateLeft: n (-want, +got): \n%s", diff)
	}
}

func TestParser_RotateRight(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

	p.root = newTree(
		&Node[string]{
			Value: "P",
			Children: []*Node[string]{
				{
					Value: "Q",
					Children: []*Node[string]{
						{
							Value: "A",
						},
						{
							Value: "B",
						},
					},
				},
				{
					Value: "C",
				},
			},
		},
	)

	// Current node is Node P
	p.node = p.root.Children[0]

	newSubRoot := p.RotateRight()

	// Expect that Q is rotated above P.
	expectedRoot := newTree(
		&Node[string]{
			Value: "Q",
			Children: []*Node[string]{
				{
					Value: "A",
				},
				{
					Value: "P",
					Children: []*Node[string]{
						{
							Value: "B",
						},
						{
							Value: "C",
						},
					},
				},
			},
		},
	)
	expectedSubRoot := expectedRoot.Children[0]

	// The new parent Node Q is returned.
	if diff := cmp.Diff(expectedSubRoot, newSubRoot); diff != "" {
		t.Fatalf("RotateRight: (-want, +got): \n%s", diff)
	}
	// Current node is set to Q
	if diff := cmp.Diff(expectedSubRoot, p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	// Full tree has expected values.
	if diff := cmp.Diff(expectedRoot, p.root); diff != "" {
		t.Errorf("p.root (-want, +got): \n%s", diff)
	}
}

func TestParser_RotateRight_root(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

	p.root = addParent(
		&Node[string]{
			Value: "P",
			Children: []*Node[string]{
				{
					Value: "Q",
					Children: []*Node[string]{
						{
							Value: "A",
						},
						{
							Value: "B",
						},
					},
				},
				{
					Value: "C",
				},
			},
		},
	)
	// Current node is Node P
	p.node = p.root

	newSubRoot := p.RotateRight()

	// Expect that Q is rotated above P.
	expectedSubRoot := addParent(
		&Node[string]{
			Value: "Q",
			Children: []*Node[string]{
				{
					Value: "A",
				},
				{
					Value: "P",
					Children: []*Node[string]{
						{
							Value: "B",
						},
						{
							Value: "C",
						},
					},
				},
			},
		},
	)

	// The new parent Node Q is returned.
	if diff := cmp.Diff(expectedSubRoot, newSubRoot); diff != "" {
		t.Fatalf("RotateRight: (-want, +got): \n%s", diff)
	}
	// Current node is set to Q
	if diff := cmp.Diff(expectedSubRoot, p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	// Full tree has expected values.
	if diff := cmp.Diff(expectedSubRoot, p.root); diff != "" {
		t.Errorf("p.root (-want, +got): \n%s", diff)
	}
}

func TestParser_RotateRight_empty(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

	n := p.RotateRight()
	if diff := cmp.Diff(p.node, n); diff != "" {
		t.Fatalf("RotateRight: n (-want, +got): \n%s", diff)
	}
}

func TestParser_SetLeft(t *testing.T) {
	t.Parallel()

	root := &Node[string]{Value: "root"}
	left := &Node[string]{Value: "left"}
	newLeft := &Node[string]{Value: "newLeft"}

	oldLeft := root.SetLeft(left)
	if oldLeft != nil {
		t.Errorf("oldLeft: want nil got %v", oldLeft)
	}

	if root.Left() != left {
		t.Errorf("root.Left(): want %v got %v", left, root.Left())
	}

	if left.Parent != root {
		t.Errorf("left.Parent: want %v got %v", root, left.Parent)
	}

	oldLeft = root.SetLeft(newLeft)
	if oldLeft != left {
		t.Errorf("oldLeft: want %v got %v", left, oldLeft)
	}

	if root.Left() != newLeft {
		t.Errorf("root.Left(): want %v got %v", newLeft, root.Left())
	}

	if newLeft.Parent != root {
		t.Errorf("newLeft.Parent: want %v got %v", root, newLeft.Parent)
	}
}

func TestParser_SetRight(t *testing.T) {
	t.Parallel()

	root := &Node[string]{Value: "root"}
	right := &Node[string]{Value: "right"}
	newRight := &Node[string]{Value: "newRight"}

	oldRight := root.SetRight(right)
	if oldRight != nil {
		t.Errorf("oldRight: want nil got %v", oldRight)
	}

	if root.Right() != right {
		t.Errorf("root.Right(): want %v got %v", right, root.Right())
	}

	if right.Parent != root {
		t.Errorf("right.Parent: want %v got %v", root, right.Parent)
	}

	oldRight = root.SetRight(newRight)
	if oldRight != right {
		t.Errorf("oldRight: want %v got %v", right, oldRight)
	}

	if root.Right() != newRight {
		t.Errorf("root.Right(): want %v got %v", newRight, root.Right())
	}

	if newRight.Parent != root {
		t.Errorf("newRight.Parent: want %v got %v", root, newRight.Parent)
	}
}
