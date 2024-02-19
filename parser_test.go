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
	"log"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ianlewis/runeio"
)

var errTreeMismatchValue = errors.New("trees node values do not match")

const (
	debugPrint = false
	inputABC   = "A B C"
)

func newTree[V comparable](n *Node[V]) *Tree[V] {
	t := &Tree[V]{
		Root: n,
	}
	addParent(t.Root)
	return t
}

func addParent[V comparable](n *Node[V]) {
	for _, c := range n.Children {
		c.Parent = n
		addParent(c)
	}
}

// compareTrees returns true if two trees are equivalent by comparing the
// value of each node in both trees.
// TODO(#31): Remove compareTrees.
func compareTrees[T comparable](t1, t2 *Tree[T]) (bool, error) {
	if diff := cmp.Diff(t2, t1); diff != "" {
		return false, fmt.Errorf("%w (-want, +got): \n%s", errTreeMismatchValue, diff)
	}
	return true, nil
}

// debugPrintTreeNodes walks tree nodes and prints a visualization of the tree
// when debugPrint is true.
func debugPrintTreeNodes[T comparable](n int, node *Node[T]) {
	if !debugPrint {
		return
	}

	log.Printf(strings.Repeat(" ", n)+"Value: %+v", node.Value)

	for _, c := range node.Children {
		debugPrintTreeNodes[T](n+1, c)
	}
}

// testLexer creates and returns a lexer.
func testLexer(t *testing.T, input string) (<-chan *Lexeme, context.CancelFunc) {
	t.Helper()

	l := NewLexer(runeio.NewReader(strings.NewReader(input)), &wordState{})

	ctx, cancel := context.WithCancel(context.Background())
	return l.Lex(ctx), cancel
}

// testParse creates and runs a lexer, and returns the parse tree.
func testParse(t *testing.T, input string) (*Tree[string], error) {
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
			case "op":
				p.Push(lexeme.Value)
			default:
				p.Node(lexeme.Value)
			}
		}
		return nil, nil
	}

	ctx := context.Background()
	tree, err := p.Parse(ctx, pFn)
	return tree, err
}

func TestParser_new(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	tree := p.Tree()
	if tree == nil {
		t.Errorf("tree should not be nil")
	} else {
		if tree.Root == nil {
			t.Errorf("tree root should not be nil")
		}
		if tree.Root.Parent != nil {
			t.Errorf("tree root parent should be nil")
		}
		if len(tree.Root.Children) != 0 {
			t.Errorf("tree root should have no children")
		}
		if tree.Root.Value != "" {
			t.Errorf("tree root value should be blank")
		}
	}
}

// TestParser_parse_op2 builds a tree of 2-child operations.
func TestParser_parse_op2(t *testing.T) {
	t.Parallel()

	input := "op 1 op 2 3"

	tree, err := testParse(t, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	debugPrintTreeNodes[string](0, tree.Root)

	// Does the tree look as expected?

	expectedTree := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
					Value: "op",
					Children: []*Node[string]{
						{
							Value: "1",
						},
						{
							Value: "op",
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
				},
			},
		},
	)

	got, expErr := compareTrees[string](tree, expectedTree)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want := true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	// Does checking the shape of the tree work as expected?

	treeUnexpShape := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
					Value: "op",
					Children: []*Node[string]{
						{
							Value: "1",
						},
						{
							Value: "op",
						},
						{
							Value: "2",
						},
						{
							Value: "3",
						},
					},
				},
			},
		},
	)

	got, unexpShpErr := compareTrees[string](tree, treeUnexpShape)
	if unexpShpErr == nil {
		t.Errorf("error unexpected trees match: %s", unexpShpErr)
	}
	want = false
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	// Does checking values in the the tree work as expected?

	treeUnexpValue := &Tree[string]{
		Root: &Node[string]{
			Children: []*Node[string]{
				{
					Value: "op",
					Children: []*Node[string]{
						{
							Value: "1",
						},
						{
							Value: "op",
							Children: []*Node[string]{
								{
									Value: "2",
								},
								{
									Value: "4",
								},
							},
						},
					},
				},
			},
		},
	}
	debugPrintTreeNodes[string](0, treeUnexpValue.Root)

	got, unexpValErr := compareTrees[string](tree, treeUnexpValue)
	if unexpValErr == nil {
		t.Errorf("error unexpected trees match: %s", unexpValErr)
	}
	want = false
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}
}

func TestParser_AdoptSibling(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	p.Push("op")
	p.Node("1")
	p.Node("2")
	p.Push("foo")
	tree1 := p.Tree()
	debugPrintTreeNodes[string](0, tree1.Root)

	expected1 := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
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
			},
		},
	)

	got, expErr := compareTrees[string](tree1, expected1)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want := true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	adoptNode, adoptErr := p.AdoptSibling()
	wantNode := p.Pos()
	var wantErr error
	if adoptNode != wantNode {
		t.Errorf("AdoptSibling node: want: %v, got: %v", wantNode, adoptNode)
	}
	if !errors.Is(adoptErr, wantErr) {
		t.Errorf("AdoptSibling err: want: %v, got: %v", wantErr, adoptErr)
	}

	tree2 := p.Tree()
	debugPrintTreeNodes[string](0, tree2.Root)

	expected2 := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
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
				},
			},
		},
	)

	got, expErr = compareTrees[string](tree2, expected2)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want = true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}
}

func TestParser_AdoptSibling_empty(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	gotNode, gotErr := p.AdoptSibling()
	var wantNode *Node[string]
	wantErr := ErrMissingRequiredNode
	if gotNode != wantNode {
		t.Errorf("empty tree AdoptSibling node: want: %v, got: %v", wantNode, gotNode)
	}
	if !errors.Is(gotErr, wantErr) {
		t.Errorf("empty tree AdoptSibling err: want: %v, got: %v", wantErr, gotErr)
	}
}

func TestParser_AdoptSibling_notfound(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	p.Push("op")
	p.Push("foo")

	gotNode, gotErr := p.AdoptSibling()
	var wantNode *Node[string]
	wantErr := ErrMissingRequiredNode
	if gotNode != wantNode {
		t.Errorf("no sibling AdoptSibling node: want: %v, got: %v", wantNode, gotNode)
	}
	if !errors.Is(gotErr, wantErr) {
		t.Errorf("no sibling AdoptSibling err: want: %v, got: %v", wantErr, gotErr)
	}
}

func TestParser_NextPeek(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	// expect to read the first lexeme "A"
	lexeme1 := p.Next()
	if lexeme1 == nil {
		t.Errorf("lexeme should not be nil")
	} else {
		got := lexeme1.Value
		want := "A"
		if got != want {
			t.Errorf("lexeme match: want: %v, got: %v", want, got)
		}
	}

	// expect to peek at second lexeme "B" without consuming it
	lexeme2 := p.Peek()
	if lexeme2 == nil {
		t.Errorf("lexeme should not be nil")
	} else {
		got := lexeme2.Value
		want := "B"
		if got != want {
			t.Errorf("lexeme match: want: %v, got: %v", want, got)
		}
	}

	// expect to read the second lexeme "B" because it was not consumed
	lexeme3 := p.Next()
	if lexeme3 == nil {
		t.Errorf("lexeme should not be nil")
	} else {
		got := lexeme3.Value
		want := "B"
		if got != want {
			t.Errorf("lexeme match: want: %v, got: %v", want, got)
		}
	}

	// expect to read the last lexeme "C"
	lexeme4 := p.Next()
	if lexeme4 == nil {
		t.Errorf("lexeme should not be nil")
	} else {
		got := lexeme4.Value
		want := "C"
		if got != want {
			t.Errorf("lexeme match: want: %v, got: %v", want, got)
		}
	}

	// expected end of lexemes
	lexeme5 := p.Next()
	if lexeme5 != nil {
		t.Errorf("lexeme should be nil")
	}
}

func TestParser_Node(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	p.Node("1")
	tree1 := p.Tree()

	expected1 := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
					Value: "1",
				},
			},
		},
	)

	got, expErr := compareTrees[string](tree1, expected1)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want := true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	p.Node("2")
	tree2 := p.Tree()
	debugPrintTreeNodes[string](0, tree2.Root)

	expected2 := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
					Value: "1",
				},
				{
					Value: "2",
				},
			},
		},
	)

	got, expErr = compareTrees[string](tree2, expected2)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want = true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	_ = p.Peek() // Peek should not affect the Node position.
	p.Node("3")
	tree3 := p.Tree()
	debugPrintTreeNodes[string](0, tree3.Root)

	expected3 := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
					Value: "1",
				},
				{
					Value: "2",
				},
				{
					Value: "3",
				},
			},
		},
	)

	if diff := cmp.Diff(expected3, tree3); diff != "" {
		t.Errorf("%v (-want, +got): \n%s", errTreeMismatchValue, diff)
	}
}

func TestParser_Pop(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	p.Push("1")
	p.Pop()
	p.Push("2")
	p.Pop()
	p.Push("3")
	tree1 := p.Tree()
	debugPrintTreeNodes[string](0, tree1.Root)

	expected1 := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
					Value: "1",
				},
				{
					Value: "2",
				},
				{
					Value: "3",
				},
			},
		},
	)

	if diff := cmp.Diff(expected1, tree1); diff != "" {
		t.Errorf("unexpected tree (-want, +got): \n%s", diff)
	}
}

func TestParser_Pos(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	p.Push("1")
	got := p.Pos().Value
	want := "1"
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	p.Push("2")
	got = p.Pos().Value
	want = "2"
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	p.Push("3")
	got = p.Pos().Value
	want = "3"
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	p.Pop()
	got = p.Pos().Value
	want = "2"
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}
}

func TestParser_Push(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	p.Push("1")
	tree1 := p.Tree()

	expected1 := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
					Value: "1",
				},
			},
		},
	)

	if diff := cmp.Diff(expected1, tree1); diff != "" {
		t.Errorf("unexpected tree (-want, +got): \n%s", diff)
	}

	p.Push("2")
	tree2 := p.Tree()

	root2 := &Node[string]{}
	tree2node1 := &Node[string]{
		Parent: root2,
		Value:  "1",
	}
	root2.Children = append(root2.Children, tree2node1)

	tree2node2 := &Node[string]{
		Parent: tree2node1,
		Value:  "2",
	}
	tree2node1.Children = append(tree2node1.Children, tree2node2)
	expected2 := &Tree[string]{
		Root: root2,
	}

	if diff := cmp.Diff(expected2, tree2); diff != "" {
		t.Errorf("unexpected tree (-want, +got): \n%s", diff)
	}
}

func TestParser_RotateLeft(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	p.Push("op")
	p.Node("1")
	p.Node("2")
	tree1 := p.Tree()
	debugPrintTreeNodes[string](0, tree1.Root)

	expected1 := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
					Value: "op",
					Children: []*Node[string]{
						{
							Value: "1",
						},
						{
							Value: "2",
						},
					},
				},
			},
		},
	)

	got, expErr := compareTrees[string](tree1, expected1)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want := true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	p.Push("foo")
	rotNode, rotErr := p.RotateLeft()
	wantNode := p.Pos()
	var wantErr error
	if rotNode != wantNode {
		t.Errorf("RotateLeft node: want: %v, got: %v", wantNode, rotNode)
	}
	if !errors.Is(rotErr, wantErr) {
		t.Errorf("RotateLeft err: want: %v, got: %v", wantErr, rotErr)
	}

	p.Node("3")
	tree2 := p.Tree()
	debugPrintTreeNodes[string](0, tree2.Root)

	expected2 := newTree(
		&Node[string]{
			Children: []*Node[string]{
				{
					Value: "foo",
					Children: []*Node[string]{
						{
							Value: "op",
							Children: []*Node[string]{
								{
									Value: "1",
								},
								{
									Value: "2",
								},
							},
						},
						{
							Value: "3",
						},
					},
				},
			},
		},
	)

	got, expErr = compareTrees[string](tree2, expected2)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want = true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}
}

func TestParser_RotateLeft_empty(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes, cancel := testLexer(t, input)
	defer cancel()

	p := NewParser[string](lexemes)

	gotNode, gotErr := p.RotateLeft()
	var wantNode *Node[string]
	wantErr := ErrMissingRequiredNode
	if gotNode != wantNode {
		t.Errorf("empty tree RotateLeft node: want: %v, got: %v", wantNode, gotNode)
	}
	if !errors.Is(gotErr, wantErr) {
		t.Errorf("empty tree RotateLeft err: want: %v, got: %v", wantErr, gotErr)
	}
}
