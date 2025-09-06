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

// Package lexer defines a Lexer interface and related types for tokenizing
// input streams.
package lexer

import (
	"context"
	"fmt"
	"strconv"
)

// TokenType is a user-defined Token type.
type TokenType int

// TokenTypeEOF indicates an EOF token signaling the end of input.
var TokenTypeEOF TokenType = -1

type Position struct {
	// Filename is the name of the file being read. It can be empty if the
	// input is not from a file.
	Filename string

	// Offset is the byte offset in the input stream, starting at 0.
	Offset int

	// Line is the line number in the input stream, starting at 1.
	Line int

	// Column is the column number in the line, starting at 1. It counts
	// characters in the line, including whitespace and newlines.
	Column int
}

// String returns a string representation of the Position.
func (p Position) String() string {
	if p.Filename != "" {
		return p.Filename + ":" + strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Column)
	}
	return strconv.Itoa(p.Line) + ":" + strconv.Itoa(p.Column)
}

// Token is a tokenized input which can be emitted by a Lexer.
type Token struct {
	// Type is the Token's type.
	Type TokenType

	// Value is the Token's value.
	Value string

	// Pos is the position in the byte stream where the Token was found.
	Pos Position
}

// String returns a string representation of the Token.
func (t Token) String() string {
	value := t.Value
	if t.Type == TokenTypeEOF {
		value = "<EOF>"
	}
	return fmt.Sprintf("%s: %s", t.Pos, value)
}

// Lexer is an interface that defines the methods for a lexer that tokenizes
// input streams. It reads from an input stream and emits [Token]s.
type Lexer interface {
	// NextToken returns the next token from the input. If there are no more
	// tokens, the context is canceled, or an error occurs, it returns a Token
	// with Type set to [TokenTypeEOF].
	NextToken(context.Context) *Token

	// Err returns the error encountered by the lexer, if any. If the error
	// encountered is [io.EOF], it will return nil.
	Err() error
}
