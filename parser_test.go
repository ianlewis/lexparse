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
	"github.com/google/go-cmp/cmp/cmpopts"
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

func TestParser_AdoptSibling(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

	p.root = newTree(
		&Node[string]{
			Value: "op",
			Children: []*Node[string]{
				{
					Value: "1",
				},
				{
					Value: "2",
				},
				{
					Value: "foo",
				},
			},
		},
	)
	// Current node is "foo"
	p.node = p.root.Children[0].Children[2]

	// expected tree after AdoptSibling
	expectedRoot := newTree(&Node[string]{
		Value: "op",
		Children: []*Node[string]{
			{
				Value: "1",
			},
			{
				Value: "foo",
				Children: []*Node[string]{
					{
						Value: "2",
					},
				},
			},
		},
	})

	n, err := p.AdoptSibling()
	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("AdoptSibling: err (-want, +got): \n%s", diff)
	}
	if diff := cmp.Diff(n, expectedRoot.Children[0].Children[1]); diff != "" {
		t.Fatalf("AdoptSibling: (-want, +got): \n%s", diff)
	}

	if diff := cmp.Diff(expectedRoot, p.root); diff != "" {
		t.Fatalf("AdoptSibling: p.root (-want, +got): \n%s", diff)
	}
	// Current node is "foo"
	if diff := cmp.Diff(expectedRoot.Children[0].Children[1], p.node); diff != "" {
		t.Fatalf("AdoptSibling: p.node (-want, +got): \n%s", diff)
	}
}

func TestParser_AdoptSibling_empty(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

	n, err := p.AdoptSibling()
	if diff := cmp.Diff(ErrMissingRequiredNode, err, cmpopts.EquateErrors()); diff != "" {
		t.Fatalf("AdoptSibling: err (-want, +got): \n%s", diff)
	}

	if diff := cmp.Diff((*Node[string])(nil), n); diff != "" {
		t.Fatalf("AdoptSibling: n (-want, +got): \n%s", diff)
	}
}

func TestParser_AdoptSibling_notfound(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

	p.root = newTree(
		&Node[string]{
			Value: "op",
			Children: []*Node[string]{
				{
					Value: "foo",
				},
			},
		},
	)
	// Current node is "foo"
	p.node = p.root.Children[0].Children[0]

	n, err := p.AdoptSibling()
	if diff := cmp.Diff(ErrMissingRequiredNode, err, cmpopts.EquateErrors()); diff != "" {
		t.Fatalf("AdoptSibling: err (-want, +got): \n%s", diff)
	}

	if diff := cmp.Diff((*Node[string])(nil), n); diff != "" {
		t.Fatalf("AdoptSibling: n (-want, +got): \n%s", diff)
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
			Value: "A",
			Children: []*Node[string]{
				{
					Value: "A1",
				},
				{
					Value: "A2",
					Children: []*Node[string]{
						{
							Value: "A2-1",
						},
						{
							Value: "A2-2",
						},
					},
				},
			},
		},
	)
	// Current node is Node A2
	p.node = p.root.Children[0].Children[1]

	n, err := p.RotateLeft()

	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("RotateLeft: err (-want, +got): \n%s", diff)
	}

	// Expect that A2 is rotated above A.
	expectedRoot := newTree(
		&Node[string]{
			Value: "A2",
			Children: []*Node[string]{
				{
					Value: "A2-1",
				},
				{
					Value: "A2-2",
				},
				{
					Value: "A",
					Children: []*Node[string]{
						{
							Value: "A1",
						},
					},
				},
			},
		},
	)

	// The new parent Node A2 is returned.
	if diff := cmp.Diff(expectedRoot.Children[0], n); diff != "" {
		t.Fatalf("RotateLeft: (-want, +got): \n%s", diff)
	}
	// Current node is set to A2
	if diff := cmp.Diff(expectedRoot.Children[0], p.node); diff != "" {
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

	p.root = newTree(
		&Node[string]{
			Value: "A",
			Children: []*Node[string]{
				{
					Value: "B",
				},
			},
		},
		&Node[string]{
			Value: "C",
		},
	)
	// Current node is Node A
	p.node = p.root.Children[0]

	n, err := p.RotateLeft()

	if diff := cmp.Diff(nil, err); diff != "" {
		t.Fatalf("RotateLeft: err (-want, +got): \n%s", diff)
	}

	// Expect that A is the new root node and the old root node has been
	// rotated under it.
	expectedRoot := addParent(
		&Node[string]{
			Value: "A",
			Children: []*Node[string]{
				{
					Value: "B",
				},
				{
					Value: "", // Default root node's value.
					Children: []*Node[string]{
						{
							Value: "C",
						},
					},
				},
			},
		},
	)

	// Node A is returned.
	if diff := cmp.Diff(expectedRoot, n); diff != "" {
		t.Fatalf("RotateLeft: (-want, +got): \n%s", diff)
	}
	// Current node is set to A
	if diff := cmp.Diff(expectedRoot, p.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	// Full tree has expected values.
	if diff := cmp.Diff(expectedRoot, p.root); diff != "" {
		t.Errorf("p.root (-want, +got): \n%s", diff)
	}
}

func TestParser_RotateLeft_empty(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil)

	n, err := p.RotateLeft()
	if diff := cmp.Diff(ErrMissingRequiredNode, err, cmpopts.EquateErrors()); diff != "" {
		t.Fatalf("AdoptSibling: err (-want, +got): \n%s", diff)
	}
	if diff := cmp.Diff((*Node[string])(nil), n); diff != "" {
		t.Fatalf("AdoptSibling: n (-want, +got): \n%s", diff)
	}
}
