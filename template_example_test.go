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
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/ianlewis/lexparse"
	"github.com/ianlewis/lexparse/lexer"
)

const (
	lexTypeText lexer.TokenType = iota
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
	// nodeTypeSeq is a node whose children are various text,if,var nodes in order.
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

func lexTokenErr(err error, t *lexer.Token) error {
	return fmt.Errorf("%w: %s", err, t)
}

// lexText tokenizes normal text.
func lexText(_ context.Context, l *lexer.CustomLexer) (lexer.LexState, error) {
	for {
		p := l.PeekN(2)
		switch string(p) {
		case tokenBlockStart, tokenVarStart:
			if l.Width() > 0 {
				l.Emit(lexTypeText)
			}
			return lexer.LexStateFn(lexCode), nil
		default:
		}

		// Advance the input.
		if !l.Advance() {
			// End of input. Emit the text up to this point.
			if l.Width() > 0 {
				l.Emit(lexTypeText)
			}
			return nil, nil
		}
	}
}

// lexCode tokenizes template code.
func lexCode(_ context.Context, l *lexer.CustomLexer) (lexer.LexState, error) {
	// Consume whitespace and discard it.
	// TODO(#94): use backtracking
	for {
		if !unicode.IsSpace(l.Peek()) {
			break
		}
		if !l.Discard() {
			// End of input
			return nil, nil
		}
	}

	rn := l.Peek()
	switch {
	case idenRegexp.MatchString(string(rn)):
		return lexer.LexStateFn(lexIden), nil
	case symbolRegexp.MatchString(string(rn)):
		return lexer.LexStateFn(lexSymbol), nil
	default:
		return nil, fmt.Errorf("code: %w: %q; line: %d, column: %d", errRune,
			rn, l.Pos().Line, l.Pos().Column)
	}
}

// lexIden tokenizes identifiers (e.g. variable names).
func lexIden(_ context.Context, l *lexer.CustomLexer) (lexer.LexState, error) {
	for {
		if rn := l.Peek(); !idenRegexp.MatchString(string(rn)) {
			l.Emit(lexTypeIdentifier)
			return lexer.LexStateFn(lexCode), nil
		}

		if !l.Advance() {
			return nil, nil
		}
	}
}

// lexSymbol tokenizes template symbols (e.g. {%, {{, }}, %}).
func lexSymbol(_ context.Context, l *lexer.CustomLexer) (lexer.LexState, error) {
	for {
		switch l.Token() {
		case tokenVarStart:
			l.Emit(lexTypeVarStart)
			return lexer.LexStateFn(lexCode), nil
		case tokenVarEnd:
			l.Emit(lexTypeVarEnd)
			return lexer.LexStateFn(lexText), nil
		case tokenBlockStart:
			l.Emit(lexTypeBlockStart)
			return lexer.LexStateFn(lexCode), nil
		case tokenBlockEnd:
			l.Emit(lexTypeBlockEnd)
			return lexer.LexStateFn(lexText), nil
		default:
			if rn := l.Peek(); !symbolRegexp.MatchString(string(rn)) {
				return nil, fmt.Errorf("symbol: %w: %q; line: %d, column: %d",
					errRune, rn, l.Pos().Line, l.Pos().Column)
			}
		}

		if !l.Advance() {
			return nil, nil
		}
	}
}

// parseRoot updates the root node to be a sequence block.
func parseRoot(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	p.Replace(&tmplNode{
		typ: nodeTypeSeq,
	})

	p.PushState(lexparse.ParseStateFn(parseSeq))
	return nil
}

// parseSeq delegates to another parse function based on token type.
func parseSeq(ctx context.Context, p *lexparse.Parser[*tmplNode]) error {
	token := p.Peek(ctx)

	switch token.Type {
	case lexTypeText:
		p.PushState(lexparse.ParseStateFn(parseText))
	case lexTypeVarStart:
		p.PushState(lexparse.ParseStateFn(parseVarStart))
	case lexTypeBlockStart:
		p.PushState(lexparse.ParseStateFn(parseBlockStart))
	}

	return nil
}

// parseText handles normal text.
func parseText(ctx context.Context, p *lexparse.Parser[*tmplNode]) error {
	token := p.Next(ctx)

	// Emit a text node.
	p.Node(&tmplNode{
		typ:  nodeTypeText,
		text: token.Value,
	})

	// Return to handling a sequence.
	p.PushState(lexparse.ParseStateFn(parseSeq))

	return nil
}

// parseVarStart handles var start (e.g. '{{').
func parseVarStart(ctx context.Context, p *lexparse.Parser[*tmplNode]) error {
	// Consume the var start token.
	_ = p.Next(ctx)

	p.PushState(
		lexparse.ParseStateFn(parseVar),
		lexparse.ParseStateFn(parseVarEnd),
	)

	return nil
}

// parseVar handles replacement variables (e.g. the 'var' in {{ var }}).
func parseVar(ctx context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(ctx); token.Type {
	case lexTypeIdentifier:
		// Validate the variable name.
		if !idenRegexp.MatchString(token.Value) {
			return lexTokenErr(fmt.Errorf("%w: invalid variable name", errIdentifier), token)
		}

		// Add a variable node.
		_ = p.Node(&tmplNode{
			typ:     nodeTypeVar,
			varName: token.Value,
		})

		return nil
	case lexer.TokenTypeEOF:
		return fmt.Errorf("%w: parsing variable name", io.ErrUnexpectedEOF)
	default:
		return lexTokenErr(errIdentifier, token)
	}
}

// parseVarEnd handles var end (e.g. '}}').
func parseVarEnd(ctx context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(ctx); token.Type {
	case lexTypeVarEnd:
		// Go back to parsing template init state.
		p.PushState(lexparse.ParseStateFn(parseSeq))
		return nil
	case lexer.TokenTypeEOF:
		return fmt.Errorf("%w: unclosed variable, expected %q", io.ErrUnexpectedEOF, tokenVarEnd)
	default:
		return fmt.Errorf("%w: expected %q", lexTokenErr(errIdentifier, token), tokenVarEnd)
	}
}

// parseBranch handles the start if conditional block.
func parseBranch(ctx context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(ctx); token.Type {
	case lexTypeIdentifier:
		if token.Value != tokenIf {
			return fmt.Errorf("%w: expected %q", errIdentifier, tokenIf)
		}

		// Add a branch node.
		_ = p.Push(&tmplNode{
			typ: nodeTypeBranch,
		})

		p.PushState(
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
	case lexer.TokenTypeEOF:
		return fmt.Errorf("%w: expected %q", io.ErrUnexpectedEOF, tokenIf)
	default:
		return lexTokenErr(errIdentifier, token)
	}
}

// parseIf handles the if body.
func parseIf(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	// Add an if body sequence node.
	_ = p.Push(&tmplNode{
		typ: nodeTypeSeq,
	})

	p.PushState(lexparse.ParseStateFn(parseSeq))
	return nil
}

// parseElse handles an else (or endif) block.
func parseElse(ctx context.Context, p *lexparse.Parser[*tmplNode]) error {
	token := p.Peek(ctx)

	switch token.Type {
	case lexTypeIdentifier:
		// Validate we are at a sequence node.
		if cur := p.Pos(); cur.Value.typ != nodeTypeSeq {
			return lexTokenErr(errIdentifier, token)
		}
	case lexer.TokenTypeEOF:
		return fmt.Errorf("%w: unclosed if block, looking for %q or %q", io.ErrUnexpectedEOF, tokenElse, tokenEndif)
	default:
		return lexTokenErr(errIdentifier, token)
	}

	switch token.Value {
	case tokenElse:
		// Consume the token.
		_ = p.Next(ctx)

		// Climb the tree back to the conditional.
		p.Climb()

		// Validate that we are in a conditional and there isn't already an else branch.
		if cur := p.Pos(); cur.Value.typ != nodeTypeBranch || len(cur.Children) != 2 {
			return lexTokenErr(errIdentifier, token)
		}

		// Add an else sequence node to the conditional.
		_ = p.Push(&tmplNode{
			typ: nodeTypeSeq,
		})

		p.PushState(
			// Parse the '%}'
			lexparse.ParseStateFn(parseBlockEnd),

			// parse the else sequence block.
			lexparse.ParseStateFn(parseSeq),

			// parse the endif.
			lexparse.ParseStateFn(parseEndif),
		)
	case tokenEndif:
		p.PushState(lexparse.ParseStateFn(parseEndif))
	default:
		return lexTokenErr(fmt.Errorf("%w: looking for %q or %q", errIdentifier, tokenElse, tokenEndif), token)
	}

	return nil
}

// parseEndif handles either an endif block.
func parseEndif(ctx context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(ctx); token.Type {
	case lexTypeIdentifier:
		if token.Value != tokenEndif {
			return lexTokenErr(fmt.Errorf("%w: looking for %q", errIdentifier, tokenEndif), token)
		}

		// Climb out of the sequence node.
		p.Climb()

		// Climb out of the branch node.
		p.Climb()

		p.PushState(
			// parse the '%}'
			lexparse.ParseStateFn(parseBlockEnd),

			// Go back to parsing a sequence.
			lexparse.ParseStateFn(parseSeq),
		)
		return nil
	case lexer.TokenTypeEOF:
		return fmt.Errorf("%w: looking for %q", io.ErrUnexpectedEOF, tokenEndif)
	default:
		return lexTokenErr(errIdentifier, token)
	}
}

// parseBlockStart handles the start of a template block '{%'.
func parseBlockStart(ctx context.Context, p *lexparse.Parser[*tmplNode]) error {
	// Validate the block start token.
	switch token := p.Next(ctx); token.Type {
	case lexTypeBlockStart:
		// OK
		fmt.Fprintln(os.Stderr, token.Value)
	case lexer.TokenTypeEOF:
		return fmt.Errorf("%w: expected %q", io.ErrUnexpectedEOF, tokenBlockStart)
	default:
		return lexTokenErr(errIdentifier, token)
	}

	// Validate the command token.
	token := p.Peek(ctx)
	switch token.Type {
	case lexTypeIdentifier:
		// OK
	case lexer.TokenTypeEOF:
		return fmt.Errorf("%w: expected %q", io.ErrUnexpectedEOF, tokenBlockStart)
	default:
		return fmt.Errorf("%w: expected %q, %q, or %q",
			lexTokenErr(errIdentifier, token), tokenIf, tokenElse, tokenEndif)
	}

	// Handle the block command.
	switch token.Value {
	case tokenIf:
		p.PushState(lexparse.ParseStateFn(parseBranch))
	case tokenElse, tokenEndif:
		// NOTE: parseElse,parseEndif should already be on the stack.
	default:
		return lexTokenErr(
			fmt.Errorf("%w: expected %q, %q, or %q", errIdentifier, tokenIf, tokenElse, tokenEndif), token)
	}

	return nil
}

// parseBlockEnd handles the end of a template block '%}'.
func parseBlockEnd(ctx context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(ctx); token.Type {
	case lexTypeBlockEnd:
		return nil
	case lexer.TokenTypeEOF:
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

func execNode(root *lexparse.Node[*tmplNode], data map[string]string, b *strings.Builder) error {
	for _, n := range root.Children {
		switch n.Value.typ {
		case nodeTypeText:
			// Write raw text to the output.
			b.WriteString(n.Value.text)
		case nodeTypeVar:
			// Replace templated variables with given data.
			b.WriteString(data[n.Value.varName])
		case nodeTypeBranch:
			// condition sanity check
			if len(n.Children) < 2 {
				panic(fmt.Sprintf("invalid branch: %#v", n))
			}

			// Get the condition.
			cond := n.Children[0]
			// Condition sanity check
			if cond.Value.typ != nodeTypeVar {
				panic(fmt.Sprintf("invalid branch condition: %#v", cond))
			}

			v, err := strconv.ParseBool(data[n.Value.varName])
			if (err == nil && v) || (err != nil && data[n.Value.varName] != "") {
				if err := execNode(n.Children[0], data, b); err != nil {
					return err
				}
			} else {
				if err := execNode(n.Children[1], data, b); err != nil {
					return err
				}
			}
		case nodeTypeSeq:
			if err := execNode(n, data, b); err != nil {
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
// can be passed with a data map to the Execute function to interpret the template
// and retrieve a final result.
//
// This example includes some best practices for error handling, such as
// including line and column numbers in error messages.
func Example_templateEngine() {
	r := strings.NewReader(`Hello, {% if subject %}{{ subject }}{% else %}World{% endif %}!`)

	t, err := lexparse.LexParse(
		context.Background(),
		lexer.NewCustomLexer(r, lexer.LexStateFn(lexText)),
		lexparse.ParseStateFn(parseRoot),
	)
	if err != nil {
		panic(err)
	}
	txt, err := Execute(t, map[string]string{"subject": "世界"})
	if err != nil {
		panic(err)
	}
	fmt.Print(txt)

	// Output: Hello, 世界!
}
