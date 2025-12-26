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
	"strings"

	"github.com/ianlewis/lexparse"
)

const (
	// lexINITypeIden represents an identifier token (key or section name).
	lexINITypeIden lexparse.TokenType = iota

	// lexINITypeOper represents an operator token.
	lexINITypeOper

	// lexINITypeValue represents a property value token.
	lexINITypeValue

	// lexINITypeComment represents a comment token.
	lexINITypeComment
)

type iniNodeType int

const (
	// iniNodeTypeRoot represents the root node of the INI parse tree.
	iniNodeTypeRoot iniNodeType = iota

	// iniNodeTypeSection represents a section node in the INI parse tree.
	iniNodeTypeSection

	// iniNodeTypeProperty represents a property node in the INI parse tree.
	iniNodeTypeProperty
)

type iniNode struct {
	typ iniNodeType

	// sectionName is only used for section nodes.
	sectionName string

	// propertyName and propertyValue are only used for property nodes.
	propertyName  string
	propertyValue string
}

func (n *iniNode) String() string {
	switch n.typ {
	case iniNodeTypeRoot:
		return "root"
	case iniNodeTypeSection:
		return fmt.Sprintf("[%s]", n.sectionName)
	case iniNodeTypeProperty:
		return fmt.Sprintf("%s = %s", n.propertyName, n.propertyValue)
	default:
		return "<Unknown>"
	}
}

var iniIdenRegexp = regexp.MustCompile(`^[A-Za-z0-9]+$`)

var (
	errINIIdentifier   = errors.New("unexpected identifier")
	errINISectionName  = errors.New("invalid section name")
	errINIPropertyName = errors.New("invalid property name")
)

// lexINI is the initial lexer state for INI files.
//
//nolint:ireturn // returning the generic interface is needed to return the previous value.
func lexINI(_ context.Context, lexer *lexparse.CustomLexer) (lexparse.LexState, error) {
	for {
		rn := lexer.Peek()
		switch rn {
		case ' ', '\t', '\r', '\n':
			lexer.Discard()
		case '[', ']', '=':
			return lexparse.LexStateFn(lexINIOper), nil
		case ';', '#':
			return lexparse.LexStateFn(lexINIComment), nil
		case lexparse.EOF:
			return nil, io.EOF
		default:
			return lexparse.LexStateFn(lexINIIden), nil
		}
	}
}

// lexINIOper lexes an operator token.
//
//nolint:ireturn // returning the generic interface is needed to return the previous value.
func lexINIOper(_ context.Context, lexer *lexparse.CustomLexer) (lexparse.LexState, error) {
	oper := lexer.NextRune()
	lexer.Emit(lexINITypeOper)

	if oper == '=' {
		return lexparse.LexStateFn(lexINIValue), nil
	}

	return lexparse.LexStateFn(lexINI), nil
}

// lexINIIden lexes an identifier token (section name or property key).
//
//nolint:ireturn // returning the generic interface is needed to return the previous value.
func lexINIIden(_ context.Context, lexer *lexparse.CustomLexer) (lexparse.LexState, error) {
	if next := lexer.Find([]string{"]", "="}); next != "" {
		lexer.Emit(lexINITypeIden)
		return lexparse.LexStateFn(lexINIOper), nil
	}

	return nil, io.ErrUnexpectedEOF
}

// lexINIValue lexes a property value token.
//
//nolint:ireturn // returning the generic interface is needed to return the previous value.
func lexINIValue(_ context.Context, lexer *lexparse.CustomLexer) (lexparse.LexState, error) {
	lexer.Find([]string{";", "\n"})
	lexer.Emit(lexINITypeValue)

	return lexparse.LexStateFn(lexINI), nil
}

// lexINIComment lexes a comment token.
//
//nolint:ireturn // returning the generic interface is needed to return the previous value.
func lexINIComment(_ context.Context, lexer *lexparse.CustomLexer) (lexparse.LexState, error) {
	lexer.Find([]string{"\n"})
	lexer.Emit(lexINITypeComment)

	return lexparse.LexStateFn(lexINI), nil
}

// iniTokenErr formats an error message with token context.
func iniTokenErr(err error, t *lexparse.Token) error {
	return fmt.Errorf("%w: %q, line %d, column %d", err,
		t.Value, t.Start.Line, t.Start.Column)
}

// parseINIInit is the initial parser state for INI files.
func parseINIInit(_ context.Context, p *lexparse.Parser[*iniNode]) error {
	// Replace the root node with a new root node.
	_ = p.Replace(&iniNode{
		typ: iniNodeTypeRoot,
	})

	// Create the empty section node for the global section.
	_ = p.Push(&iniNode{
		typ:         iniNodeTypeSection,
		sectionName: "",
	})

	p.PushState(lexparse.ParseStateFn(parseINI))

	return nil
}

// parseINI parses the top-level structure of an INI file.
func parseINI(ctx context.Context, p *lexparse.Parser[*iniNode]) error {
	t := p.Peek(ctx)

	switch t.Type {
	case lexINITypeOper:
		p.PushState(lexparse.ParseStateFn(parseSection))
	case lexINITypeIden:
		p.PushState(lexparse.ParseStateFn(parseProperty))
	case lexINITypeComment:
		_ = p.Next(ctx) // Discard comment
		p.PushState(lexparse.ParseStateFn(parseINI))
	case lexparse.TokenTypeEOF:
		return nil
	default:
		return iniTokenErr(errINIIdentifier, t)
	}

	return nil
}

// parseSection parses a section header.
func parseSection(ctx context.Context, parser *lexparse.Parser[*iniNode]) error {
	openBracket := parser.Next(ctx)
	if openBracket.Type != lexINITypeOper || openBracket.Value != "[" {
		return iniTokenErr(errINIIdentifier, openBracket)
	}

	sectionToken := parser.Next(ctx)
	if sectionToken.Type != lexINITypeIden {
		return iniTokenErr(errINIIdentifier, sectionToken)
	}

	closeBracket := parser.Next(ctx)
	if closeBracket.Type != lexINITypeOper || closeBracket.Value != "]" {
		return iniTokenErr(errINIIdentifier, closeBracket)
	}

	sectionName := strings.TrimSpace(sectionToken.Value)

	// Validate the section name.
	if !iniIdenRegexp.MatchString(sectionName) {
		return iniTokenErr(errINISectionName, sectionToken)
	}

	// Create a new node for the section and push it onto the parse tree.
	// The current node is now the new section node.
	_ = parser.Climb()
	_ = parser.Push(&iniNode{
		typ:         iniNodeTypeSection,
		sectionName: sectionName,
	})

	parser.PushState(lexparse.ParseStateFn(parseINI))

	return nil
}

// parseProperty parses a property key-value pair.
func parseProperty(ctx context.Context, parser *lexparse.Parser[*iniNode]) error {
	keyToken := parser.Next(ctx)
	if keyToken.Type != lexINITypeIden {
		return iniTokenErr(errINIIdentifier, keyToken)
	}

	keyName := strings.TrimSpace(keyToken.Value)

	// Validate the property name.
	if !iniIdenRegexp.MatchString(keyName) {
		return iniTokenErr(errINIPropertyName, keyToken)
	}

	eqToken := parser.Next(ctx)
	if eqToken.Type != lexINITypeOper || eqToken.Value != "=" {
		return iniTokenErr(errINIIdentifier, eqToken)
	}

	valueToken := parser.Next(ctx)
	if valueToken.Type != lexINITypeValue {
		return iniTokenErr(errINIIdentifier, valueToken)
	}

	// Create a new node for the property and add it to the current section.
	parser.Node(&iniNode{
		typ:           iniNodeTypeProperty,
		propertyName:  keyName,
		propertyValue: strings.TrimSpace(valueToken.Value),
	})

	parser.PushState(lexparse.ParseStateFn(parseINI))

	return nil
}

// Example_iniParser demonstrates parsing a simple INI file. It does not support
// nested sections, or escape sequences.
func Example_iniParser() {
	r := strings.NewReader(`; last modified 1 April 2001 by John Doe
[owner]
name = John Doe
organization = Acme Widgets Inc.

[database]
; use IP address in case network name resolution is not working
server = 192.0.2.62
port = 143
file = "payroll.dat"
`)

	// Produces a tree representation of the INI file.
	// Each child of the root node is a section node, which in turn
	// has property nodes as children. The global section is represented
	// as a section node with an empty name.
	tree, err := lexparse.LexParse(
		context.Background(),
		lexparse.NewCustomLexer(r, lexparse.LexStateFn(lexINI)),
		lexparse.ParseStateFn(parseINIInit),
	)
	if err != nil {
		panic(err)
	}

	fmt.Print(tree)

	// Output:
	// root (0:0)
	// ├── [] (0:0)
	// ├── [owner] (2:7)
	// │   ├── name = John Doe (3:7)
	// │   └── organization = Acme Widgets Inc. (4:15)
	// └── [database] (6:10)
	//     ├── server = 192.0.2.62 (8:9)
	//     ├── port = 143 (9:7)
	//     └── file = "payroll.dat" (10:7)
}
