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
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/ianlewis/runeio"

	"github.com/ianlewis/lexparse"
)

const (
	lexTypeText lexparse.LexemeType = iota
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

func tokenErr(err error, t *lexparse.Lexeme) error {
	return fmt.Errorf("%w: %q: line: %d, column: %d", err, t.Value, t.Line, t.Column)
}

// lexText tokenizes normal text.
func lexText(_ context.Context, l *lexparse.Lexer) (lexparse.LexState, error) {
	for {
		p, err := l.Peek(2)
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("lexing text: %w", err)
		}
		switch string(p) {
		case tokenBlockStart, tokenVarStart:
			if l.Width() > 0 {
				l.Emit(lexTypeText)
			}
			return lexparse.LexStateFn(lexCode), nil
		default:
		}

		// Advance the input.
		if _, err := l.Advance(1); err != nil {
			if errors.Is(err, io.EOF) {
				// End of input. Emit the text up to this point.
				if l.Width() > 0 {
					l.Emit(lexTypeText)
				}
				return nil, nil
			}
			return nil, fmt.Errorf("lexing text: %w", err)
		}
	}
}

// lexCode tokenizes template code.
func lexCode(_ context.Context, l *lexparse.Lexer) (lexparse.LexState, error) {
	// Consume whitespace and discard it.
	// TODO(#94): use backtracking
	for {
		rn, err := l.Peek(1)
		// TODO(#95): update EOF check.
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("lexing code: %w", err)
		}
		if !unicode.IsSpace(rn[0]) {
			break
		}
		if _, err := l.Discard(len(rn)); err != nil {
			return nil, fmt.Errorf("lexing code: %w", err)
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
			rn, err := l.Peek(1)
			// TODO(#95): update EOF check.
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("lexing code: %w", err)
			}
			if unicode.IsSpace(rn[0]) {
				l.Emit(lexTypeIdentifier)
				return lexparse.LexStateFn(lexCode), nil
			}
		}

		if _, err := l.Advance(1); err != nil {
			return nil, fmt.Errorf("lexing code: %w", err)
		}
	}
}

// parseRoot updates the root node to be a code block.
func parseRoot(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	fmt.Fprintf(os.Stderr, "parseRoot: %#v\n", p.Pos().Value)

	p.Replace(&tmplNode{
		typ: nodeTypeCode,
	})

	p.PushState(lexparse.ParseStateFn(parseCode))
	return nil
}

// parseCode delegates to another parse function based on lexeme type.
func parseCode(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	fmt.Fprintf(os.Stderr, "parseCode: %#v\n", p.Pos().Value)

	token := p.Peek()
	// TODO(#95): Remove nil check.
	if token == nil {
		return nil
	}

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
	default:
		// NOTE: This shouldn't happen.
		return fmt.Errorf("%w: %v", errType, token.Type)
	}
}

// parseText handles normal text.
func parseText(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	fmt.Fprintf(os.Stderr, "parseText: %#v\n", p.Pos().Value)

	// Get the next lexeme from the parser.
	l := p.Next()
	// TODO(#95): Remove nil check.
	if l == nil {
		return nil
	}

	// Emit a text node.
	p.Node(&tmplNode{
		typ:  nodeTypeText,
		text: l.Value,
	})

	// Return to handling code.
	p.PushState(lexparse.ParseStateFn(parseCode))
	return nil
}

var varNameRegexp = regexp.MustCompile(`[a-zA-Z]+[a-zA-Z0-9]*`)

// parseVarStart handles var start (e.g. '{{').
func parseVarStart(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	fmt.Fprintf(os.Stderr, "parseVarStart: %#v\n", p.Pos().Value)

	token := p.Next()
	// TODO(#95): Remove nil check.
	if token == nil {
		return tokenErr(errUnclosedVar, token)
	}

	// Validate the var start token type.
	if token.Type != lexTypeVarStart {
		return tokenErr(errUnexpectedIdentifier, token)
	}

	p.PushState(
		lexparse.ParseStateFn(parseVar),
		lexparse.ParseStateFn(parseVarEnd),
	)

	return nil
}

// parseVar handles replacement variables (e.g. the 'var' in {{ var }}).
func parseVar(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	fmt.Fprintf(os.Stderr, "parseVar: %#v\n", p.Pos().Value)

	nameToken := p.Next()

	// Validate the name token's type.
	if nameToken.Type != lexTypeIdentifier {
		return tokenErr(errUnexpectedIdentifier, nameToken)
	}

	// Validate the variable name.
	if !varNameRegexp.MatchString(nameToken.Value) {
		return tokenErr(errUnexpectedIdentifier, nameToken)
	}

	// Add a variable node.
	_ = p.Node(&tmplNode{
		typ:     nodeTypeVar,
		varName: nameToken.Value,
	})

	return nil
}

// parseVarEnd handles var end (e.g. '}}').
func parseVarEnd(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	fmt.Fprintf(os.Stderr, "parseVarEnd: %#v\n", p.Pos().Value)

	// Validate the end variable.
	token := p.Next()
	// TODO(#95): Remove nil check.
	if token == nil {
		return tokenErr(errUnclosedVar, token)
	}

	// Validate the var end token type.
	if token.Type != lexTypeVarEnd {
		return tokenErr(errUnexpectedIdentifier, token)
	}

	// Go back to parsing code.
	p.PushState(lexparse.ParseStateFn(parseCode))
	return nil
}

// parseBranch handles the start if conditional block.
func parseBranch(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	fmt.Fprintf(os.Stderr, "parseBranch: %#v\n", p.Pos().Value)

	// Validate the if token
	token := p.Next()
	// TODO(#95): Remove nil check.
	if token == nil {
		return io.ErrUnexpectedEOF
	}

	if token.Type != lexTypeIdentifier || token.Value != tokenIf {
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
}

// parseIf handles the if body.
func parseIf(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	fmt.Fprintf(os.Stderr, "parseIf: %#v\n", p.Pos().Value)

	// Add an if body code node.
	_ = p.Push(&tmplNode{
		typ: nodeTypeCode,
	})

	p.PushState(lexparse.ParseStateFn(parseCode))
	return nil
}

// parseElse handles either an else block.
func parseElse(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	fmt.Fprintf(os.Stderr, "parseElse: %#v\n", p.Pos().Value)

	// Validate the token
	token := p.Peek()
	// TODO(#95): Remove nil check.
	if token == nil {
		return io.ErrUnexpectedEOF
	}

	if token.Type != lexTypeIdentifier {
		return tokenErr(errUnexpectedIdentifier, token)
	}

	if cur := p.Pos(); cur.Value.typ != nodeTypeCode {
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
	fmt.Fprintf(os.Stderr, "parseEndif: %#v\n", p.Pos().Value)

	// Validate the endif token
	token := p.Next()
	// TODO(#95): Remove nil check.
	if token == nil {
		return io.ErrUnexpectedEOF
	}

	if token.Type != lexTypeIdentifier || token.Value != tokenEndif {
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
}

// parseBlockStart handles the start of a template block '{%'.
func parseBlockStart(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	// Validate the block start token
	next := p.Next()
	// TODO(#95): Remove nil check.
	if next == nil {
		return io.ErrUnexpectedEOF
	}
	if next.Type != lexTypeBlockStart {
		return fmt.Errorf("%w: %v", errUnexpectedIdentifier, next.Value)
	}

	// Validate the command.
	token := p.Peek()
	// TODO(#95): Remove nil check.
	if token == nil {
		return fmt.Errorf("%w: %v", errUnclosedBlock, token.Value)
	}
	if token.Type != lexTypeIdentifier {
		return fmt.Errorf("%w: %v", errUnclosedBlock, token.Value)
	}

	// validate the location is a code block.
	if cur := p.Pos(); cur.Value.typ != nodeTypeCode {
		return fmt.Errorf("%w: %v", errUnexpectedIdentifier, token.Value)
	}

	switch token.Value {
	case tokenIf:
		p.PushState(lexparse.ParseStateFn(parseBranch))
	case tokenElse, tokenEndif:
		// NOTE: parseElse,parseEndif should already be on the stack.
	default:
		return fmt.Errorf("%w: %v", errUnexpectedIdentifier, token.Value)
	}

	return nil
}

// parseBlockEnd handles the end of a template block '%}'.
func parseBlockEnd(_ context.Context, p *lexparse.Parser[*tmplNode]) error {
	fmt.Fprintf(os.Stderr, "parseBlockEnd: %#v\n", p.Pos().Value)

	// Validate the block end token
	next := p.Next()
	// TODO(#95): Remove nil check.
	if next == nil {
		return io.ErrUnexpectedEOF
	}
	if next.Type != lexTypeBlockEnd {
		return tokenErr(errUnexpectedIdentifier, next)
	}
	return nil
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
	lexemes := make(chan *lexparse.Lexeme, 1024)
	r := runeio.NewReader(strings.NewReader("Hello, {% if subject %}{{ subject }}{% else %}World{% endif %}!"))

	t, err := lexparse.LexParse(
		context.Background(),
		lexparse.NewLexer(r, lexemes, lexparse.LexStateFn(lexText)),
		lexparse.NewParser(lexemes, lexparse.ParseStateFn(parseRoot)),
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
