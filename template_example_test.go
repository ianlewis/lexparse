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

	"github.com/ianlewis/runeio"

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
	errType                 = errors.New("unexpected type")
	errUnexpectedIdentifier = errors.New("unexpected identifier")
	errUnclosedVar          = errors.New("unclosed variable")
	errUnclosedBlock        = errors.New("unclosed block")
	ErrUnclosedAction       = errors.New("unclosed action")
)

type nodeType int

const (
	// nodeTypeCode is a node whose children are various text,if,var nodes in order.
	nodeTypeCode nodeType = iota

	// nodeTypeText is a leaf node comprised of text.
	nodeTypeText

	// nodeTypeBranch is a binary node whose first child is the 'if' code
	// node and second is the 'else' code node.
	nodeTypeBranch

	// nodeTypeVar nodes are variable leaf nodes.
	nodeTypeVar
)

type tmplNode struct {
	typ     nodeType
	varName string
	text    string
}

func tokenErr(err error, t *lexparse.Token) error {
	return fmt.Errorf("%w: %q: line: %d, column: %d", err, t.Value, t.Line, t.Column)
}

// lexText tokenizes normal text.
func lexText(_ context.Context, l *lexparse.Lexer) (lexparse.LexState, error) {
	for {
		p := l.PeekN(2)
		switch string(p) {
		case tokenBlockStart, tokenVarStart:
			if l.Width() > 0 {
				l.Emit(lexTypeText)
			}
			return lexparse.LexStateFn(lexCode), nil
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
func lexCode(_ context.Context, l *lexparse.Lexer) (lexparse.LexState, error) {
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

	for {
		switch l.Token() {
		case tokenVarStart:
			l.Emit(lexTypeVarStart)
			return lexparse.LexStateFn(lexCode), nil
		case tokenVarEnd:
			l.Emit(lexTypeVarEnd)
			return lexparse.LexStateFn(lexText), nil
		case tokenBlockStart:
			l.Emit(lexTypeBlockStart)
			return lexparse.LexStateFn(lexCode), nil
		case tokenBlockEnd:
			l.Emit(lexTypeBlockEnd)
			return lexparse.LexStateFn(lexText), nil
		default:
			if unicode.IsSpace(l.Peek()) {
				l.Emit(lexTypeIdentifier)
				return lexparse.LexStateFn(lexCode), nil
			}
		}

		if !l.Advance() {
			return nil, nil
		}
	}
}

// parseRoot updates the root node to be a code block.
func parseRoot(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	p.Replace(&tmplNode{
		typ: nodeTypeCode,
	})

	p.PushState(lexparse.ParseStateFn(parseCode))
	return nil
}

// parseCode delegates to another parse function based on token type.
func parseCode(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	token := p.Peek()

	// Validate that we are in a code node.
	if cur := p.Pos(); cur.Value.typ != nodeTypeCode {
		return tokenErr(errUnexpectedIdentifier, token)
	}

	switch token.Type {
	case lexTypeText:
		p.PushState(lexparse.ParseStateFn(parseText))
		return nil
	case lexTypeVarStart:
		p.PushState(lexparse.ParseStateFn(parseVarStart))
		return nil
	case lexTypeBlockStart:
		p.PushState(lexparse.ParseStateFn(parseBlockStart))
		return nil
	case lexparse.TokenTypeEOF:
		return nil
	default:
		// NOTE: This shouldn't happen.
		return tokenErr(errUnexpectedIdentifier, token)
	}
}

// parseText handles normal text.
func parseText(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(); token.Type {
	case lexTypeText:
		// Emit a text node.
		p.Node(&tmplNode{
			typ:  nodeTypeText,
			text: token.Value,
		})

		// Return to handling code.
		p.PushState(lexparse.ParseStateFn(parseCode))

		return nil
	case lexparse.TokenTypeEOF:
		return io.ErrUnexpectedEOF
	default:
		// NOTE: This shouldn't happen.
		return fmt.Errorf("%w: %v", errType, token.Type)
	}
}

var varNameRegexp = regexp.MustCompile(`[a-zA-Z]+[a-zA-Z0-9]*`)

// parseVarStart handles var start (e.g. '{{').
func parseVarStart(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(); token.Type {
	case lexTypeVarStart:
		p.PushState(
			lexparse.ParseStateFn(parseVar),
			lexparse.ParseStateFn(parseVarEnd),
		)
		return nil
	case lexparse.TokenTypeEOF:
		return io.ErrUnexpectedEOF
	default:
		return tokenErr(errUnexpectedIdentifier, token)
	}
}

// parseVar handles replacement variables (e.g. the 'var' in {{ var }}).
func parseVar(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(); token.Type {

	case lexTypeIdentifier:
		// Validate the variable name.
		if !varNameRegexp.MatchString(token.Value) {
			return tokenErr(errUnexpectedIdentifier, token)
		}

		// Add a variable node.
		_ = p.Node(&tmplNode{
			typ:     nodeTypeVar,
			varName: token.Value,
		})

		return nil
	case lexparse.TokenTypeEOF:
		return io.ErrUnexpectedEOF
	default:
		return tokenErr(errUnexpectedIdentifier, token)
	}
}

// parseVarEnd handles var end (e.g. '}}').
func parseVarEnd(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(); token.Type {
	case lexTypeVarEnd:
		// Go back to parsing code.
		p.PushState(lexparse.ParseStateFn(parseCode))
		return nil
	case lexparse.TokenTypeEOF:
		return io.ErrUnexpectedEOF
	default:
		return tokenErr(errUnexpectedIdentifier, token)
	}
}

// parseBranch handles the start if conditional block.
func parseBranch(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(); token.Type {
	case lexTypeIdentifier:
		if token.Value != tokenIf {
			return tokenErr(errUnexpectedIdentifier, token)
		}

		// Add a branch node.
		_ = p.Push(&tmplNode{
			typ: nodeTypeBranch,
		})

		p.PushState(
			// The first child will be the condition expression.
			// Currently only a simple variable is supported.
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
		return io.ErrUnexpectedEOF
	default:
		return tokenErr(errUnexpectedIdentifier, token)
	}
}

// parseIf handles the if body.
func parseIf(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	// Add an if body code node.
	_ = p.Push(&tmplNode{
		typ: nodeTypeCode,
	})

	p.PushState(lexparse.ParseStateFn(parseCode))
	return nil
}

// parseElse handles an else (or endif) block.
func parseElse(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	token := p.Peek()

	switch token.Type {
	case lexTypeIdentifier:
		// Validate we are at a code node.
		if cur := p.Pos(); cur.Value.typ != nodeTypeCode {
			return tokenErr(errUnexpectedIdentifier, token)
		}
	case lexparse.TokenTypeEOF:
		return io.ErrUnexpectedEOF
	default:
		return tokenErr(errUnexpectedIdentifier, token)
	}

	switch token.Value {
	case tokenElse:
		// Consume the token.
		_ = p.Next()

		// Climb the tree back to the conditional.
		p.Climb()

		// Validate that we are in a conditional and there isn't already an else branch.
		if cur := p.Pos(); cur.Value.typ != nodeTypeBranch || len(cur.Children) != 2 {
			return tokenErr(errUnexpectedIdentifier, token)
		}

		// Add an else code node to the conditional.
		_ = p.Push(&tmplNode{
			typ: nodeTypeCode,
		})

		p.PushState(
			// Parse the '%}'
			lexparse.ParseStateFn(parseBlockEnd),

			// parse the else code block.
			lexparse.ParseStateFn(parseCode),

			// parse the endif.
			lexparse.ParseStateFn(parseEndif),
		)
	case tokenEndif:
		p.PushState(lexparse.ParseStateFn(parseEndif))
	default:
		return tokenErr(errUnexpectedIdentifier, token)
	}

	return nil
}

// parseEndif handles either an endif block.
func parseEndif(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(); token.Type {
	case lexTypeIdentifier:
		if token.Value != tokenEndif {
			return tokenErr(errUnexpectedIdentifier, token)
		}

		// Climb out of the code node.
		p.Climb()

		// Climb out of the branch node.
		p.Climb()

		p.PushState(
			// parse the '%}'
			lexparse.ParseStateFn(parseBlockEnd),

			// Go back to parsing code.
			lexparse.ParseStateFn(parseCode),
		)
		return nil
	case lexparse.TokenTypeEOF:
		return io.ErrUnexpectedEOF
	default:
		return tokenErr(errUnexpectedIdentifier, token)
	}
}

// parseBlockStart handles the start of a template block '{%'.
func parseBlockStart(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	// Validate the block start token.
	switch token := p.Next(); token.Type {
	case lexTypeBlockStart:
		// OK
	case lexparse.TokenTypeEOF:
		return io.ErrUnexpectedEOF
	default:
		return tokenErr(errUnexpectedIdentifier, token)
	}

	// Validate the command token.
	token := p.Peek()
	switch token.Type {
	case lexTypeIdentifier:
		// OK
	case lexparse.TokenTypeEOF:
		return io.ErrUnexpectedEOF
	default:
		return tokenErr(errUnexpectedIdentifier, token)
	}

	// validate the location is a code block.
	if cur := p.Pos(); cur.Value.typ != nodeTypeCode {
		return tokenErr(errUnexpectedIdentifier, token)
	}

	// Handle the block command.
	switch token.Value {
	case tokenIf:
		p.PushState(lexparse.ParseStateFn(parseBranch))
	case tokenElse, tokenEndif:
		// NOTE: parseElse,parseEndif should already be on the stack.
	default:
		return tokenErr(errUnexpectedIdentifier, token)
	}

	return nil
}

// parseBlockEnd handles the end of a template block '%}'.
func parseBlockEnd(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	switch token := p.Next(); token.Type {
	case lexTypeBlockEnd:
		return nil
	case lexparse.TokenTypeEOF:
		return io.ErrUnexpectedEOF
	default:
		return tokenErr(errUnexpectedIdentifier, token)
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
			if len(n.Children) < 2 {
				panic(fmt.Sprintf("invalid branch: %#v", n))
			}

			// Get the condition.
			cond := n.Children[0]
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
		case nodeTypeCode:
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
	tokens := make(chan *lexparse.Token, 1024)
	r := runeio.NewReader(strings.NewReader(`Hello, {% if subject %}{{ subject }}{% else %}World{% endif %}!`))

	t, err := lexparse.LexParse(
		context.Background(),
		lexparse.NewLexer(r, tokens, lexparse.LexStateFn(lexText)),
		lexparse.NewParser(tokens, lexparse.ParseStateFn(parseRoot)),
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
