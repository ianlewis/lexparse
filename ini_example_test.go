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
func lexINI(_ context.Context, cursor *lexparse.CustomLexerCursor) (lexparse.LexState, error) {
	for {
		rn := cursor.Peek()
		switch rn {
		case ' ', '\t', '\r', '\n':
			cursor.Discard()
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
func lexINIOper(_ context.Context, cursor *lexparse.CustomLexerCursor) (lexparse.LexState, error) {
	oper := cursor.NextRune()
	cursor.Emit(lexINITypeOper)

	if oper == '=' {
		return lexparse.LexStateFn(lexINIValue), nil
	}

	return lexparse.LexStateFn(lexINI), nil
}

// lexINIIden lexes an identifier token (section name or property key).
//
//nolint:ireturn // returning the generic interface is needed to return the previous value.
func lexINIIden(_ context.Context, cursor *lexparse.CustomLexerCursor) (lexparse.LexState, error) {
	if next := cursor.Find([]string{"]", "="}); next != "" {
		cursor.Emit(lexINITypeIden)
		return lexparse.LexStateFn(lexINIOper), nil
	}

	return nil, io.ErrUnexpectedEOF
}

// lexINIValue lexes a property value token.
//
//nolint:ireturn // returning the generic interface is needed to return the previous value.
func lexINIValue(_ context.Context, cursor *lexparse.CustomLexerCursor) (lexparse.LexState, error) {
	cursor.Find([]string{";", "\n"})
	cursor.Emit(lexINITypeValue)

	return lexparse.LexStateFn(lexINI), nil
}

// lexINIComment lexes a comment token.
//
//nolint:ireturn // returning the generic interface is needed to return the previous value.
func lexINIComment(_ context.Context, cursor *lexparse.CustomLexerCursor) (lexparse.LexState, error) {
	cursor.Find([]string{"\n"})
	cursor.Emit(lexINITypeComment)

	return lexparse.LexStateFn(lexINI), nil
}

// iniTokenErr formats an error message with token context.
func iniTokenErr(err error, t *lexparse.Token) error {
	return fmt.Errorf("%w: %q, line %d, column %d", err,
		t.Value, t.Start.Line, t.Start.Column)
}

// parseINIInit is the initial parser state for INI files.
func parseINIInit(ctx *lexparse.ParserContext[*iniNode]) error {
	// Replace the root node with a new root node.
	_ = ctx.Replace(&iniNode{
		typ: iniNodeTypeRoot,
	})

	// Create the empty section node for the global section.
	_ = ctx.Push(&iniNode{
		typ:         iniNodeTypeSection,
		sectionName: "",
	})

	ctx.PushState(lexparse.ParseStateFn(parseINI))

	return nil
}

// parseINI parses the top-level structure of an INI file.
func parseINI(ctx *lexparse.ParserContext[*iniNode]) error {
	t := ctx.Peek()

	switch t.Type {
	case lexINITypeOper:
		ctx.PushState(lexparse.ParseStateFn(parseSection))
	case lexINITypeIden:
		ctx.PushState(lexparse.ParseStateFn(parseProperty))
	case lexINITypeComment:
		_ = ctx.Next() // Discard comment
		ctx.PushState(lexparse.ParseStateFn(parseINI))
	case lexparse.TokenTypeEOF:
		return nil
	default:
		return iniTokenErr(errINIIdentifier, t)
	}

	return nil
}

// parseSection parses a section header.
func parseSection(ctx *lexparse.ParserContext[*iniNode]) error {
	openBracket := ctx.Next()
	if openBracket.Type != lexINITypeOper || openBracket.Value != "[" {
		return iniTokenErr(errINIIdentifier, openBracket)
	}

	sectionToken := ctx.Next()
	if sectionToken.Type != lexINITypeIden {
		return iniTokenErr(errINIIdentifier, sectionToken)
	}

	closeBracket := ctx.Next()
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
	_ = ctx.Climb()
	_ = ctx.Push(&iniNode{
		typ:         iniNodeTypeSection,
		sectionName: sectionName,
	})

	ctx.PushState(lexparse.ParseStateFn(parseINI))

	return nil
}

// parseProperty parses a property key-value pair.
func parseProperty(ctx *lexparse.ParserContext[*iniNode]) error {
	keyToken := ctx.Next()
	if keyToken.Type != lexINITypeIden {
		return iniTokenErr(errINIIdentifier, keyToken)
	}

	keyName := strings.TrimSpace(keyToken.Value)

	// Validate the property name.
	if !iniIdenRegexp.MatchString(keyName) {
		return iniTokenErr(errINIPropertyName, keyToken)
	}

	eqToken := ctx.Next()
	if eqToken.Type != lexINITypeOper || eqToken.Value != "=" {
		return iniTokenErr(errINIIdentifier, eqToken)
	}

	valueToken := ctx.Next()
	if valueToken.Type != lexINITypeValue {
		return iniTokenErr(errINIIdentifier, valueToken)
	}

	// Create a new node for the property and add it to the current section.
	ctx.Node(&iniNode{
		typ:           iniNodeTypeProperty,
		propertyName:  keyName,
		propertyValue: strings.TrimSpace(valueToken.Value),
	})

	ctx.PushState(lexparse.ParseStateFn(parseINI))

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
