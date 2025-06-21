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

package lexer

import "context"

// EOF is a rune that indicates that the lexer has finished processing.
var EOF rune = -1

// TokenType is a user-defined Token type.
type TokenType int

// TokenTypeEOF indicates an EOF token signalling the end of input.
var TokenTypeEOF TokenType = -1

// Token is a tokenized input which can be emitted by a Lexer.
type Token struct {
	// Type is the Token's type.
	Type TokenType

	// Value is the Token's value.
	Value string

	// Pos is the position in the byte stream where the Token was found.
	Pos int

	// Line is the line number where the Token was found (one-based).
	Line int

	// Column is the column in the line where the Token was found (one-based).
	Column int
}

type Lexer interface {
	// NextToken returns the next token from the input.
	NextToken(context.Context) (Token, error)
}
