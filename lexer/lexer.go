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
	Pos Position
}

// String returns a string representation of the Token.
func (t Token) String() string {
	if t.Type == TokenTypeEOF {
		return "EOF"
	}
	return t.Value
}

// Lexer is an interface that defines the methods for a lexer that tokenizes
// input streams. It reads from an input stream and emits [Token]s.
type Lexer interface {
	// NextToken returns the next token from the input. If there are no more
	// tokens, or an error occurs, it returns a Token with Type set to
	// [TokenTypeEOF].
	NextToken(context.Context) Token

	// Err returns the error encountered by the lexer, if any. If the error
	// encountered is [io.EOF], it will return nil.
	Err() error
}
