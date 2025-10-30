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

import (
	"context"
	"errors"
	"fmt"
	"io"
	"text/scanner"
)

var errScanner = errors.New("scanner error")

var (
	// TokenTypeEOF represents the end of the file.
	TokenTypeEOF = TokenType(scanner.EOF)

	// TokenTypeIdent represents an identifier.
	TokenTypeIdent = TokenType(scanner.Ident)

	// TokenTypeInt represents an integer literal.
	TokenTypeInt = TokenType(scanner.Int)

	// TokenTypeFloat represents a floating-point literal.
	TokenTypeFloat = TokenType(scanner.Float)

	// TokenTypeChar represents a character literal.
	TokenTypeChar = TokenType(scanner.Char)

	// TokenTypeString represents a string literal.
	TokenTypeString = TokenType(scanner.String)

	// TokenTypeRawString represents a raw string literal.
	TokenTypeRawString = TokenType(scanner.RawString)

	// TokenTypeComment represents a comment.
	TokenTypeComment = TokenType(scanner.Comment)
)

// ScanningLexer is a lexer that uses the text/scanner package to tokenize
// input streams.
type ScanningLexer struct {
	s *scanner.Scanner

	// err is the first error the lexer encountered.
	err error
}

// NewScanningLexer creates a new ScanningLexer that reads from the given
// [io.Reader].
func NewScanningLexer(r io.Reader) *ScanningLexer {
	l := ScanningLexer{}
	l.s = &scanner.Scanner{
		Error: func(s *scanner.Scanner, msg string) {
			if l.err == nil {
				l.err = fmt.Errorf("%w: %s: %s", errScanner, s.Position, msg)
			}
		},
	}
	l.s = l.s.Init(r)
	// Configure the scanner to be more generic and to not skip Go comments.
	l.s.Mode = scanner.ScanIdents | scanner.ScanFloats | scanner.ScanChars | scanner.ScanStrings | scanner.ScanRawStrings | scanner.ScanComments

	return &l
}

// NextToken implements [Lexer.NextToken]. It returns the next token from
// the input stream.
func (l *ScanningLexer) NextToken(_ context.Context) *Token {
	if l.err != nil {
		return l.newToken(TokenTypeEOF)
	}

	tok := l.s.Scan()
	switch tok {
	case scanner.EOF:
		return l.newToken(TokenTypeEOF)
	default:
		return l.newToken(TokenType(tok))
	}
}

// Err implements [Lexer.Err]. It returns the first error encountered by
// the lexer, if any.
func (l *ScanningLexer) Err() error {
	return l.err
}

func (l *ScanningLexer) newToken(typ TokenType) *Token {
	return &Token{
		Type:  typ,
		Value: l.s.TokenText(),
		Start: Position(l.s.Position),
		End:   Position(l.s.Pos()),
	}
}
