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
)

// addParent sets the parent reference on all children of `n`.
func addParent[V comparable](n *Node[V]) *Node[V] {
	if n != nil {
		for _, c := range n.Children {
			c.Parent = n
			_ = addParent(c)
		}
	}

	return n
}

// testParse creates and runs a lexer, and returns the root of the parse tree.
func testParse(t *testing.T, input string) (*Node[string], error) {
	t.Helper()

	l := NewCustomLexer(strings.NewReader(input), &lexWordState{})
	ctx := context.Background()

	parser := NewParser(l, ParseStateFn(func(_ context.Context, innerParser *Parser[string]) error {
		for {
			token := innerParser.Next(ctx)
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
				_ = innerParser.Climb()
			case "replace":
				_ = innerParser.Replace(token.Value)
			case "push":
				_ = innerParser.Push(token.Value)
			default:
				innerParser.Node(token.Value)
			}
		}
	}))

	root, err := parser.Parse(context.Background())

	return root, err
}

func TestParser_new(t *testing.T) {
	t.Parallel()

	p := NewParser[string](nil, nil)

	expectedRoot := &Node[string]{
		Start: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
	}
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
	expectedRoot := addParent(&Node[string]{
		Start: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
		Children: []*Node[string]{
			{
				Value: "push",
				Start: Position{
					Offset: 0,
					Line:   1,
					Column: 1,
				},
				Children: []*Node[string]{
					{
						Value: "1",
						Start: Position{
							Offset: 5,
							Line:   1,
							Column: 6,
						},
					},
					{
						Value: "push",
						Start: Position{
							Offset: 7,
							Line:   1,
							Column: 8,
						},
						Children: []*Node[string]{
							{
								Value: "2",
								Start: Position{
									Offset: 12,
									Line:   1,
									Column: 13,
								},
							},
							{
								Value: "3",
								Start: Position{
									Offset: 14,
									Line:   1,
									Column: 15,
								},
							},
						},
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
	l := NewCustomLexer(strings.NewReader(input), &lexWordState{})

	parser := NewParser[string](l, nil)

	ctx := context.Background()

	// Expect to read the first token `A`
	tokenA := parser.Next(ctx)

	wanttokenA := &Token{
		Type:  wordType,
		Value: "A",
		Start: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
		End: Position{
			Offset: 1,
			Line:   1,
			Column: 2,
		},
	}
	if diff := cmp.Diff(wanttokenA, tokenA); diff != "" {
		t.Fatalf("Next: (-want, +got): \n%s", diff)
	}

	peekTokenB := parser.Peek(ctx)

	wantTokenB := &Token{
		Type:  wordType,
		Value: "B",
		Start: Position{
			Offset: 2,
			Line:   1,
			Column: 3,
		},
		End: Position{
			Offset: 3,
			Line:   1,
			Column: 4,
		},
	}
	if diff := cmp.Diff(wantTokenB, peekTokenB); diff != "" {
		t.Fatalf("Peek: (-want, +got): \n%s", diff)
	}

	// Expect to read the second token "B" because it was not consumed
	tokenB := parser.Next(ctx)
	if diff := cmp.Diff(wantTokenB, tokenB); diff != "" {
		t.Fatalf("Peek: (-want, +got): \n%s", diff)
	}

	tokenC := parser.Next(ctx)

	wantTokenC := &Token{
		Type:  wordType,
		Value: "C",
		Start: Position{
			Offset: 4,
			Line:   1,
			Column: 5,
		},
		End: Position{
			Offset: 5,
			Line:   1,
			Column: 6,
		},
	}
	if diff := cmp.Diff(wantTokenC, tokenC); diff != "" {
		t.Fatalf("Next: (-want, +got): \n%s", diff)
	}

	// The expected end of tokens
	niltoken := parser.Next(ctx)

	tokenEOF := &Token{
		Type:  TokenTypeEOF,
		Value: "",
		Start: Position{
			Offset: 5,
			Line:   1,
			Column: 6,
		},
		End: Position{
			Offset: 5,
			Line:   1,
			Column: 6,
		},
	}
	if diff := cmp.Diff(tokenEOF, niltoken); diff != "" {
		t.Fatalf("Next: (-want, +got): \n%s", diff)
	}
}

func TestParser_Node(t *testing.T) {
	t.Parallel()

	parser := NewParser[string](nil, nil)

	child1 := parser.Node("A")
	expectedRootA := addParent(&Node[string]{
		Start: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
		Children: []*Node[string]{
			{
				Value: "A",
			},
		},
	})

	if diff := cmp.Diff(expectedRootA.Children[0], child1); diff != "" {
		t.Fatalf("Node: (-want, +got): \n%s", diff)
	}
	// Current node is still set to root.
	if diff := cmp.Diff(parser.root, parser.node); diff != "" {
		t.Errorf("p.node: (-want, +got): \n%s", diff)
	}

	child2 := parser.Node("B")
	expectedRootB := addParent(&Node[string]{
		Start: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
		Children: []*Node[string]{
			{
				Value: "A",
			},
			{
				Value: "B",
			},
		},
	})

	if diff := cmp.Diff(expectedRootB.Children[1], child2); diff != "" {
		t.Fatalf("Node: (-want, +got): \n%s", diff)
	}
	// Current node is still set to root.
	if diff := cmp.Diff(parser.root, parser.node); diff != "" {
		t.Errorf("p.node: (-want, +got): \n%s", diff)
	}

	if diff := cmp.Diff(expectedRootB, parser.root); diff != "" {
		t.Fatalf("Node: parser.root (-want, +got): \n%s", diff)
	}
}

func TestParser_ClimbPos(t *testing.T) {
	t.Parallel()

	parser := NewParser[string](nil, nil)

	parser.root = addParent(
		&Node[string]{
			Start: Position{
				Offset: 0,
				Line:   1,
				Column: 1,
			},
			Children: []*Node[string]{
				{
					Value: "A",
					Children: []*Node[string]{
						{
							Value: "B",
						},
					},
				},
			},
		},
	)
	// Current node is Node B
	parser.node = parser.root.Children[0].Children[0]

	// Climb returns Node B
	if diff := cmp.Diff(parser.root.Children[0].Children[0], parser.Climb()); diff != "" {
		t.Errorf("Climb: (-want, +got): \n%s", diff)
	}
	// Current node is set to Node A
	if diff := cmp.Diff(parser.root.Children[0], parser.node); diff != "" {
		t.Errorf("parser.node: (-want, +got): \n%s", diff)
	}
	// Pos returns Node A
	if diff := cmp.Diff(parser.root.Children[0], parser.Pos()); diff != "" {
		t.Errorf("Pos: (-want, +got): \n%s", diff)
	}

	// Climb returns Node A
	if diff := cmp.Diff(parser.root.Children[0], parser.Climb()); diff != "" {
		t.Errorf("Climb: (-want, +got): \n%s", diff)
	}
	// Current node is set to root node.
	if diff := cmp.Diff(parser.root, parser.node); diff != "" {
		t.Errorf("parser.node (-want, +got): \n%s", diff)
	}
	// Pos returns root node.
	if diff := cmp.Diff(parser.root, parser.Pos()); diff != "" {
		t.Errorf("Pos: (-want, +got): \n%s", diff)
	}

	// Climb returns root node.
	if diff := cmp.Diff(parser.root, parser.Climb()); diff != "" {
		t.Errorf("Climb: (-want, +got): \n%s", diff)
	}
	// Current node is set to root node.
	if diff := cmp.Diff(parser.root, parser.node); diff != "" {
		t.Errorf("parser.node (-want, +got): \n%s", diff)
	}
	// Pos returns root node.
	if diff := cmp.Diff(parser.root, parser.Pos()); diff != "" {
		t.Errorf("Pos: (-want, +got): \n%s", diff)
	}
}

func TestParser_Push(t *testing.T) {
	t.Parallel()

	parser := NewParser[string](nil, nil)

	valA := "A"

	expectedRootA := addParent(&Node[string]{
		Start: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
		Children: []*Node[string]{
			{
				Value: valA,
			},
		},
	})
	if diff := cmp.Diff(expectedRootA.Children[0], parser.Push(valA)); diff != "" {
		t.Errorf("Push(%q): (-want, +got): \n%s", valA, diff)
	}

	if diff := cmp.Diff(expectedRootA.Children[0], parser.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}

	if diff := cmp.Diff(expectedRootA, parser.root); diff != "" {
		t.Errorf("parser.root (-want, +got): \n%s", diff)
	}

	valB := "B"

	expectedRootB := addParent(&Node[string]{
		Start: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
		Children: []*Node[string]{
			{
				Value: "A",
				Children: []*Node[string]{
					{
						Value: "B",
					},
				},
			},
		},
	})
	if diff := cmp.Diff(expectedRootB.Children[0].Children[0], parser.Push(valB)); diff != "" {
		t.Errorf("Push(%q): (-want, +got): \n%s", valB, diff)
	}

	if diff := cmp.Diff(expectedRootB.Children[0].Children[0], parser.node); diff != "" {
		t.Errorf("parser.node (-want, +got): \n%s", diff)
	}

	if diff := cmp.Diff(expectedRootB, parser.root); diff != "" {
		t.Errorf("parser.root (-want, +got): \n%s", diff)
	}
}

func TestParser_Replace(t *testing.T) {
	t.Parallel()

	parser := NewParser[string](nil, nil)

	parser.root = addParent(&Node[string]{
		Children: []*Node[string]{
			{
				Value: "A",
				Children: []*Node[string]{
					{
						Value: "B",
					},
				},
			},
		},
	})

	// Current node is Node A
	parser.node = parser.root.Children[0]

	// Replace Node A with C
	valC := "C"
	if diff := cmp.Diff("A", parser.Replace(valC)); diff != "" {
		t.Errorf("Replace(%q): (-want, +got): \n%s", valC, diff)
	}

	expectedRoot := addParent(&Node[string]{
		Children: []*Node[string]{
			{
				Value: "C",
				Children: []*Node[string]{
					{
						Value: "B",
					},
				},
			},
		},
	})

	// Current node is set to Node `C`.
	if diff := cmp.Diff(expectedRoot.Children[0], parser.node); diff != "" {
		t.Errorf("p.node (-want, +got): \n%s", diff)
	}
	// Full tree has expected values.
	if diff := cmp.Diff(expectedRoot, parser.root); diff != "" {
		t.Errorf("parser.root (-want, +got): \n%s", diff)
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

func TestNode_String(t *testing.T) {
	t.Parallel()

	node := &Node[string]{
		Value: "root",
		Start: Position{
			Offset: 0,
			Line:   1,
			Column: 1,
		},
		Children: []*Node[string]{
			{
				Value: "child1",

				Start: Position{
					Offset: 1,
					Line:   1,
					Column: 2,
				},
				Children: []*Node[string]{
					{
						Value: "grandchild1",

						Start: Position{
							Offset: 2,
							Line:   1,
							Column: 3,
						},
					},
				},
			},
			{
				Value: "child2",
				Start: Position{
					Offset: 3,
					Line:   1,
					Column: 4,
				},
				Children: []*Node[string]{
					{
						Value: "grandchild2",
						Start: Position{
							Offset: 4,
							Line:   1,
							Column: 5,
						},
					},
				},
			},
		},
	}

	expected := `root (1:1)
├── child1 (1:2)
│   └── grandchild1 (1:3)
└── child2 (1:4)
    └── grandchild2 (1:5)
`

	if diff := cmp.Diff(expected, node.String()); diff != "" {
		t.Errorf("Node.String() (-want, +got): \n%s", diff)
	}
}
