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
	"strconv"
	"strings"
	"testing"

	"github.com/ianlewis/runeio"
)

var (
	errTreeMismatchSize  = errors.New("trees have different number of nodes")
	errTreeMismatchValue = errors.New("trees node values do not match")
)

const (
	debugPrint = false
	inputABC   = "A B C"
)

// compareTrees returns true if two trees are equivalent by comparing the
// value of each node in both trees.
func compareTrees[T any](tr1, tr2 *Tree[T]) (bool, error) {
	ch1 := make(chan string)
	ch2 := make(chan string)

	go walkTree(tr1, ch1)
	go walkTree(tr2, ch2)

	for {
		i1, more1 := <-ch1
		i2, more2 := <-ch2

		if more1 != more2 {
			return false, errTreeMismatchSize
		}
		if !more1 {
			break
		}

		if i1 != i2 {
			return false, fmt.Errorf("node values: %q, %q, %w", i1, i2, errTreeMismatchValue)
		}
	}

	return true, nil
}

// debugPrintTreeNodes walks tree nodes and prints a visualization of the tree
// when debugPrint is true.
func debugPrintTreeNodes[T any](n int, node *Node[T]) {
	if !debugPrint {
		return
	}

	log.Printf(strings.Repeat(" ", n)+"Value: %+v", node.Value)

	for _, c := range node.Children {
		debugPrintTreeNodes[T](n+1, c)
	}
}

// testLexer creates and returns a lexer.
func testLexer(t *testing.T, input string) <-chan *Lexeme {
	t.Helper()

	l := NewLexer(runeio.NewReader(strings.NewReader(input)), &wordState{})

	return l.Lex(context.Background())
}

// testParse creates and runs a lexer, and returns the parse tree.
func testParse(t *testing.T, input string) (*Tree[string], error) {
	t.Helper()

	lexemes := testLexer(t, input)

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

// walkTree walks a parse tree and sends a string value of each node to the channel.
func walkTree[T any](tr *Tree[T], ch chan<- string) {
	defer close(ch)

	doWalkTree(ch, "", tr.Root)
}

// doWalkTree is a recursive worker function used by walkTree.
func doWalkTree[T any](ch chan<- string, depth string, node *Node[T]) {
	if node == nil {
		return
	}

	message := ""
	message += fmt.Sprintf(depth+":Value: %v", node.Value)
	ch <- message

	for i, c := range node.Children {
		newDepth := depth + strconv.Itoa(i)
		doWalkTree(ch, newDepth, c)
	}
}

func TestParser_new(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes := testLexer(t, input)
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

	expectedTree := &Tree[string]{
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
									Value: "3",
								},
							},
						},
					},
				},
			},
		},
	}
	debugPrintTreeNodes[string](0, expectedTree.Root)

	got, expErr := compareTrees[string](tree, expectedTree)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want := true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	// Does checking the shape of the tree work as expected?

	treeUnexpShape := &Tree[string]{
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
	}
	debugPrintTreeNodes[string](0, treeUnexpShape.Root)

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
	lexemes := testLexer(t, input)
	p := NewParser[string](lexemes)

	p.Push("op")
	p.Node("1")
	p.Node("2")
	p.Push("foo")
	tree1 := p.Tree()
	debugPrintTreeNodes[string](0, tree1.Root)

	expected1 := &Tree[string]{
		Root: &Node[string]{
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
	}

	got, expErr := compareTrees[string](tree1, expected1)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want := true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	p.AdoptSibling()
	tree2 := p.Tree()
	debugPrintTreeNodes[string](0, tree2.Root)

	expected2 := &Tree[string]{
		Root: &Node[string]{
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
	}

	got, expErr = compareTrees[string](tree2, expected2)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want = true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}
}

func TestParser_NextPeek(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes := testLexer(t, input)
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
	lexemes := testLexer(t, input)
	p := NewParser[string](lexemes)

	p.Node("1")
	tree1 := p.Tree()
	debugPrintTreeNodes[string](0, tree1.Root)

	expected1 := &Tree[string]{
		Root: &Node[string]{
			Children: []*Node[string]{
				{
					Value: "1",
				},
			},
		},
	}

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

	expected2 := &Tree[string]{
		Root: &Node[string]{
			Children: []*Node[string]{
				{
					Value: "1",
				},
				{
					Value: "2",
				},
			},
		},
	}

	got, expErr = compareTrees[string](tree2, expected2)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want = true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}
}

func TestParser_Pop(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes := testLexer(t, input)
	p := NewParser[string](lexemes)

	p.Push("1")
	p.Pop()
	p.Push("2")
	p.Pop()
	p.Push("3")
	tree1 := p.Tree()
	debugPrintTreeNodes[string](0, tree1.Root)

	expected1 := &Tree[string]{
		Root: &Node[string]{
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
	}

	got, expErr := compareTrees[string](tree1, expected1)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want := true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}
}

func TestParser_Pos(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes := testLexer(t, input)
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
	lexemes := testLexer(t, input)
	p := NewParser[string](lexemes)

	p.Push("1")
	tree1 := p.Tree()

	expected1 := &Tree[string]{
		Root: &Node[string]{
			Children: []*Node[string]{
				{
					Value: "1",
				},
			},
		},
	}

	got, expErr := compareTrees[string](tree1, expected1)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want := true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	p.Push("2")
	tree2 := p.Tree()

	expected2 := &Tree[string]{
		Root: &Node[string]{
			Children: []*Node[string]{
				{
					Value: "1",
					Children: []*Node[string]{
						{
							Value: "2",
						},
					},
				},
			},
		},
	}

	got, expErr = compareTrees[string](tree2, expected2)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want = true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}
}

func TestParser_RotateLeft(t *testing.T) {
	t.Parallel()

	input := inputABC
	lexemes := testLexer(t, input)
	p := NewParser[string](lexemes)

	p.Push("op")
	p.Node("1")
	p.Node("2")
	tree1 := p.Tree()
	debugPrintTreeNodes[string](0, tree1.Root)

	expected1 := &Tree[string]{
		Root: &Node[string]{
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
	}

	got, expErr := compareTrees[string](tree1, expected1)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want := true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}

	p.Push("foo")
	p.RotateLeft()
	p.Node("3")
	tree2 := p.Tree()
	debugPrintTreeNodes[string](0, tree2.Root)

	expected2 := &Tree[string]{
		Root: &Node[string]{
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
	}

	got, expErr = compareTrees[string](tree2, expected2)
	if expErr != nil {
		t.Errorf("error expected trees do not match: %s", expErr)
	}
	want = true
	if got != want {
		t.Errorf("trees match: want: %v, got: %v", want, got)
	}
}
