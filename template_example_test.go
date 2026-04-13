// Copyright 2023 Google LLC
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
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/ianlewis/lexparse"
)

const (
	lexTypeText lexparse.TokenType = iota
	lexTypeBlockStart
	lexTypeBlockEnd
	lexTypeVarStart
	lexTypeVarEnd
	lexTypeIdentifier
)

const (
	tokenBlockStart = "{%"
	tokenBlockEnd   = "%}"
	tokenVarStart   = "{{"
	tokenVarEnd     = "}}"
	tokenIf         = "if"
	tokenElse       = "else"
	tokenEndif      = "endif"
)

var (
	errRune       = errors.New("unexpected rune")
	errIdentifier = errors.New("unexpected identifier")
)

// Identifier regexp.
var (
	idenRegexp   = regexp.MustCompile(`[a-zA-Z]+[a-zA-Z0-9]*`)
	symbolRegexp = regexp.MustCompile(`[{}%]+`)
)

type tmplNodeType int

const (
	// nodeTypeSeq is a node whose children are various text, if, var nodes in
	// order.
	nodeTypeSeq tmplNodeType = iota

	// nodeTypeText is a leaf node comprised of text.
	nodeTypeText

	// nodeTypeBranch is a binary node whose first child is the 'if' sequence
	// node and second is the 'else' sequence node.
	nodeTypeBranch

	// nodeTypeVar nodes are variable leaf nodes.
	nodeTypeVar
)

type tmplNode struct {
	typ tmplNodeType

	// Fields below are populated based on node type.
	varName string
	text    string
}

func (n *tmplNode) String() string {
	switch n.typ {
	case nodeTypeSeq:
		return "[]"
	case nodeTypeText:
		return fmt.Sprintf("%q", n.text)
	case nodeTypeBranch:
		return "if/else"
	case nodeTypeVar:
		return fmt.Sprintf("{{%s}}", n.varName)
	default:
		return "<Unknown>"
	}
}

func lexTokenErr(err error, t *lexparse.Token) error {
	return fmt.Errorf("%w: %s", err, t)
}

// lexText tokenizes normal text.
//
//nolint:ireturn // returning interface is required to satisfy lexparse.LexState.
func lexText(_ context.Context, cur *lexparse.LexCursor) (lexparse.LexState, error) {
	for {
		p := string(cur.PeekN(2))
		if p == tokenBlockStart || p == tokenVarStart {
			if cur.Width() > 0 {
				cur.Emit(lexTypeText)
			}

			return lexparse.LexStateFn(lexCode), nil
		}

		// Advance the input.
		if !cur.Advance() {
			// End of input. Emit the text up to this point.
			if cur.Width() > 0 {
				cur.Emit(lexTypeText)
			}

			return nil, io.EOF
		}
	}
}

// lexCode tokenizes template code.
//
//nolint:ireturn // returning interface is required to satisfy lexparse.LexState.
func lexCode(_ context.Context, cur *lexparse.LexCursor) (lexparse.LexState, error) {
	// Consume whitespace and discard it.
	// TODO(#94): use backtracking
	for unicode.IsSpace(cur.Peek()) {
		if !cur.Discard() {
			// End of input
			return nil, io.EOF
		}
	}

	rn := cur.Peek()
	switch {
	case idenRegexp.MatchString(string(rn)):
		return lexparse.LexStateFn(lexIden), nil
	case symbolRegexp.MatchString(string(rn)):
		return lexparse.LexStateFn(lexSymbol), nil
	default:
		return nil, fmt.Errorf("%w: %q; line: %d, column: %d", errRune,
			rn, cur.Pos().Line, cur.Pos().Column)
	}
}

// lexIden tokenizes identifiers (e.g. variable names).
//
//nolint:ireturn // returning interface is required to satisfy lexparse.LexState.
func lexIden(_ context.Context, cur *lexparse.LexCursor) (lexparse.LexState, error) {
	for {
		if rn := cur.Peek(); !idenRegexp.MatchString(string(rn)) {
			cur.Emit(lexTypeIdentifier)
			return lexparse.LexStateFn(lexCode), nil
		}

		if !cur.Advance() {
			return nil, io.EOF
		}
	}
}

// lexSymbol tokenizes template symbols (e.g. {%, {{, }}, %}).
//
//nolint:ireturn // returning interface is required to satisfy lexparse.LexState.
func lexSymbol(_ context.Context, cur *lexparse.LexCursor) (lexparse.LexState, error) {
	for {
		switch cur.Token() {
		case tokenVarStart:
			cur.Emit(lexTypeVarStart)
			return lexparse.LexStateFn(lexCode), nil
		case tokenVarEnd:
			cur.Emit(lexTypeVarEnd)
			return lexparse.LexStateFn(lexText), nil
		case tokenBlockStart:
			cur.Emit(lexTypeBlockStart)
			return lexparse.LexStateFn(lexCode), nil
		case tokenBlockEnd:
			cur.Emit(lexTypeBlockEnd)
			return lexparse.LexStateFn(lexText), nil
		default:
			if rn := cur.Peek(); !symbolRegexp.MatchString(string(rn)) {
				return nil, fmt.Errorf("symbol: %w: %q; line: %d, column: %d",
					errRune, rn, cur.Pos().Line, cur.Pos().Column)
			}
		}

		if !cur.Advance() {
			return nil, io.EOF
		}
	}
}

// parseRoot updates the root node to be a sequence block.
func parseRoot(_ context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	cur.Replace(&tmplNode{
		typ: nodeTypeSeq,
	})

	cur.PushState(lexparse.ParseStateFn(parseSeq))

	return nil
}

// parseSeq delegates to another parse function based on token type.
func parseSeq(ctx context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	token := cur.Peek(ctx)

	switch token.Type {
	case lexTypeText:
		cur.PushState(lexparse.ParseStateFn(parseText))
	case lexTypeVarStart:
		cur.PushState(lexparse.ParseStateFn(parseVarStart))
	case lexTypeBlockStart:
		cur.PushState(lexparse.ParseStateFn(parseBlockStart))
	default:
	}

	return nil
}

// parseText handles normal text.
func parseText(ctx context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	token := cur.Next(ctx)

	// Emit a text node.
	cur.Node(&tmplNode{
		typ:  nodeTypeText,
		text: token.Value,
	})

	// Return to handling a sequence.
	cur.PushState(lexparse.ParseStateFn(parseSeq))

	return nil
}

// parseVarStart handles var start (e.g. '{{').
func parseVarStart(ctx context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	// Consume the var start token.
	_ = cur.Next(ctx)

	cur.PushState(
		lexparse.ParseStateFn(parseVar),
		lexparse.ParseStateFn(parseVarEnd),
	)

	return nil
}

// parseVar handles replacement variables (e.g. the 'var' in {{ var }}).
func parseVar(ctx context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	switch token := cur.Next(ctx); token.Type {
	case lexTypeIdentifier:
		// Validate the variable name.
		if !idenRegexp.MatchString(token.Value) {
			return lexTokenErr(fmt.Errorf("%w: invalid variable name", errIdentifier), token)
		}

		// Add a variable node.
		_ = cur.Node(&tmplNode{
			typ:     nodeTypeVar,
			varName: token.Value,
		})

		return nil
	case lexparse.TokenTypeEOF:
		return fmt.Errorf("%w: parsing variable name", io.ErrUnexpectedEOF)
	default:
		return lexTokenErr(errIdentifier, token)
	}
}

// parseVarEnd handles var end (e.g. '}}').
func parseVarEnd(ctx context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	switch token := cur.Next(ctx); token.Type {
	case lexTypeVarEnd:
		// Go back to parsing template init state.
		cur.PushState(lexparse.ParseStateFn(parseSeq))
		return nil
	case lexparse.TokenTypeEOF:
		return fmt.Errorf("%w: unclosed variable, expected %q", io.ErrUnexpectedEOF, tokenVarEnd)
	default:
		return fmt.Errorf("%w: expected %q", lexTokenErr(errIdentifier, token), tokenVarEnd)
	}
}

// parseBranch handles the start if conditional block.
func parseBranch(ctx context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	switch token := cur.Next(ctx); token.Type {
	case lexTypeIdentifier:
		if token.Value != tokenIf {
			return fmt.Errorf("%w: expected %q", errIdentifier, tokenIf)
		}

		// Add a branch node.
		_ = cur.Push(&tmplNode{
			typ: nodeTypeBranch,
		})

		cur.PushState(
			// Parse the conditional expression.  Currently only a simple
			// variable is supported.
			lexparse.ParseStateFn(parseVar),

			// Parse the '%}'
			lexparse.ParseStateFn(parseBlockEnd),

			// Parse the if block.
			lexparse.ParseStateFn(parseIf),

			// Parse an 'else' (or 'endif')
			lexparse.ParseStateFn(parseElse),
		)

		return nil
	case lexparse.TokenTypeEOF:
		return fmt.Errorf("%w: expected %q", io.ErrUnexpectedEOF, tokenIf)
	default:
		return lexTokenErr(errIdentifier, token)
	}
}

// parseIf handles the if body.
func parseIf(_ context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	// Add an if body sequence node.
	_ = cur.Push(&tmplNode{
		typ: nodeTypeSeq,
	})

	cur.PushState(lexparse.ParseStateFn(parseSeq))

	return nil
}

// parseElse handles an else (or endif) block.
func parseElse(ctx context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	token := cur.Peek(ctx)

	switch token.Type {
	case lexTypeIdentifier:
		// Validate we are at a sequence node.
		if curPos := cur.Pos(); curPos.Value.typ != nodeTypeSeq {
			return lexTokenErr(errIdentifier, token)
		}
	case lexparse.TokenTypeEOF:
		return fmt.Errorf("%w: unclosed if block, looking for %q or %q", io.ErrUnexpectedEOF, tokenElse, tokenEndif)
	default:
		return lexTokenErr(errIdentifier, token)
	}

	switch token.Value {
	case tokenElse:
		// Consume the token.
		_ = cur.Next(ctx)

		// Climb the tree back to the conditional.
		cur.Climb()

		// Validate that we are in a conditional and there isn't already an else branch.
		if curNode := cur.Pos(); curNode.Value.typ != nodeTypeBranch || len(curNode.Children) != 2 {
			return lexTokenErr(errIdentifier, token)
		}

		// Add an else sequence node to the conditional.
		_ = cur.Push(&tmplNode{
			typ: nodeTypeSeq,
		})

		cur.PushState(
			// Parse the '%}'
			lexparse.ParseStateFn(parseBlockEnd),

			// parse the else sequence block.
			lexparse.ParseStateFn(parseSeq),

			// parse the endif.
			lexparse.ParseStateFn(parseEndif),
		)
	case tokenEndif:
		cur.PushState(lexparse.ParseStateFn(parseEndif))
	default:
		return lexTokenErr(fmt.Errorf("%w: looking for %q or %q", errIdentifier, tokenElse, tokenEndif), token)
	}

	return nil
}

// parseEndif handles either an endif block.
func parseEndif(ctx context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	switch token := cur.Next(ctx); token.Type {
	case lexTypeIdentifier:
		if token.Value != tokenEndif {
			return lexTokenErr(fmt.Errorf("%w: looking for %q", errIdentifier, tokenEndif), token)
		}

		// Climb out of the sequence node.
		cur.Climb()

		// Climb out of the branch node.
		cur.Climb()

		cur.PushState(
			// parse the '%}'
			lexparse.ParseStateFn(parseBlockEnd),

			// Go back to parsing a sequence.
			lexparse.ParseStateFn(parseSeq),
		)

		return nil
	case lexparse.TokenTypeEOF:
		return fmt.Errorf("%w: looking for %q", io.ErrUnexpectedEOF, tokenEndif)
	default:
		return lexTokenErr(errIdentifier, token)
	}
}

// parseBlockStart handles the start of a template block '{%'.
func parseBlockStart(ctx context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	// Validate the block start token.
	switch token := cur.Next(ctx); token.Type {
	case lexTypeBlockStart:
		// OK
	case lexparse.TokenTypeEOF:
		return fmt.Errorf("%w: expected %q", io.ErrUnexpectedEOF, tokenBlockStart)
	default:
		return lexTokenErr(errIdentifier, token)
	}

	// Validate the command token.
	token := cur.Peek(ctx)
	switch token.Type {
	case lexTypeIdentifier:
		// OK
	case lexparse.TokenTypeEOF:
		return fmt.Errorf("%w: expected %q", io.ErrUnexpectedEOF, tokenBlockStart)
	default:
		return fmt.Errorf("%w: expected %q, %q, or %q",
			lexTokenErr(errIdentifier, token), tokenIf, tokenElse, tokenEndif)
	}

	// Handle the block command.
	switch token.Value {
	case tokenIf:
		cur.PushState(lexparse.ParseStateFn(parseBranch))
	case tokenElse, tokenEndif:
		// NOTE: parseElse, parseEndif should already be on the stack.
	default:
		return lexTokenErr(
			fmt.Errorf("%w: expected %q, %q, or %q", errIdentifier, tokenIf, tokenElse, tokenEndif), token)
	}

	return nil
}

// parseBlockEnd handles the end of a template block '%}'.
func parseBlockEnd(ctx context.Context, cur *lexparse.ParseCursor[*tmplNode]) error {
	switch token := cur.Next(ctx); token.Type {
	case lexTypeBlockEnd:
		return nil
	case lexparse.TokenTypeEOF:
		return fmt.Errorf("%w: expected %q", io.ErrUnexpectedEOF, tokenBlockEnd)
	default:
		return lexTokenErr(fmt.Errorf("%w: expected %q", errIdentifier, tokenBlockEnd), token)
	}
}

// Execute renders the template with the given data.
func Execute(root *lexparse.Node[*tmplNode], data map[string]string) (string, error) {
	var b strings.Builder

	// Support basic boolean values.
	if _, ok := data["true"]; !ok {
		data["true"] = "true"
	}

	if _, ok := data["false"]; !ok {
		data["false"] = "false"
	}

	if err := execNode(root, data, &b); err != nil {
		return "", err
	}

	return b.String(), nil
}

func execNode(root *lexparse.Node[*tmplNode], data map[string]string, bldr *strings.Builder) error {
	for _, node := range root.Children {
		switch node.Value.typ {
		case nodeTypeText:
			// Write raw text to the output.
			bldr.WriteString(node.Value.text)
		case nodeTypeVar:
			// Replace templated variables with given data.
			bldr.WriteString(data[node.Value.varName])
		case nodeTypeBranch:
			// condition sanity check
			if len(node.Children) < 2 {
				panic(fmt.Sprintf("invalid branch: %#v", node))
			}

			// Get the condition.
			cond := node.Children[0]
			// Condition sanity check
			if cond.Value.typ != nodeTypeVar {
				panic(fmt.Sprintf("invalid branch condition: %#v", cond))
			}

			v, err := strconv.ParseBool(data[node.Value.varName])
			if (err == nil && v) || (err != nil && data[node.Value.varName] != "") {
				if err := execNode(node.Children[0], data, bldr); err != nil {
					return err
				}
			} else {
				if err := execNode(node.Children[1], data, bldr); err != nil {
					return err
				}
			}
		case nodeTypeSeq:
			if err := execNode(node, data, bldr); err != nil {
				return err
			}
		}
	}

	return nil
}

// Example_templateEngine implements a simple text templating language. The
// language replaces variables identified with double brackets
// (e.g. `{{ var }}`) with data values for those variables.
//
// LexParse is used to lex and parse the template into a parse tree. This tree
// can be passed with a data map to the Execute function to interpret the
// template and retrieve a final result.
//
// This example includes some best practices for error handling, such as
// including line and column numbers in error messages.
func Example_templateEngine() {
	r := strings.NewReader(`Hello, {% if subject %}{{ subject }}{% else %}World{% endif %}!`)

	tree, err := lexparse.LexParse(
		context.Background(),
		lexparse.NewCustomLexer(r, lexparse.LexStateFn(lexText)),
		lexparse.ParseStateFn(parseRoot),
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(tree)

	txt, err := Execute(tree, map[string]string{"subject": "世界"})
	if err != nil {
		panic(err)
	}

	fmt.Print(txt)

	// Output:
	// [] (0:0)
	// ├── "Hello, " (1:1)
	// ├── if/else (1:11)
	// │   ├── {{subject}} (1:14)
	// │   ├── [] (1:22)
	// │   │   └── {{subject}} (1:27)
	// │   └── [] (1:40)
	// │       └── "World" (1:47)
	// └── "!" (1:63)
	//
	// Hello, 世界!
}
