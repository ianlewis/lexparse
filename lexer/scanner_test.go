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
	"strings"
	"testing"
	"text/scanner"

	"github.com/google/go-cmp/cmp"
)

func TestScannerLexer_NextToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []*Token
	}{
		{
			name:  "simple input",
			input: "abc := 123 + 456",
			expected: []*Token{
				{
					Type:  TokenType(scanner.Ident),
					Value: "abc",
					Start: Position{
						Offset: 0,
						Line:   1,
						Column: 1,
					},
					End: Position{
						Offset: 3,
						Line:   1,
						Column: 4,
					},
				},
				{
					Type:  TokenType(':'),
					Value: ":",
					Start: Position{
						Offset: 4,
						Line:   1,
						Column: 5,
					},
					End: Position{
						Offset: 5,
						Line:   1,
						Column: 6,
					},
				},
				{
					Type:  TokenType('='),
					Value: "=",
					Start: Position{
						Offset: 5,
						Line:   1,
						Column: 6,
					},
					End: Position{
						Offset: 6,
						Line:   1,
						Column: 7,
					},
				},
				{
					Type:  TokenType(scanner.Int),
					Value: "123",
					Start: Position{
						Offset: 7,
						Line:   1,
						Column: 8,
					},
					End: Position{
						Offset: 10,
						Line:   1,
						Column: 11,
					},
				},
				{
					Type:  TokenType('+'),
					Value: "+",
					Start: Position{
						Offset: 11,
						Line:   1,
						Column: 12,
					},
					End: Position{
						Offset: 12,
						Line:   1,
						Column: 13,
					},
				},
				{
					Type:  TokenType(scanner.Int),
					Value: "456",
					Start: Position{
						Offset: 13,
						Line:   1,
						Column: 14,
					},
					End: Position{
						Offset: 16,
						Line:   1,
						Column: 17,
					},
				},
				{
					Type:  TokenTypeEOF,
					Value: "",
					Start: Position{
						Offset: 16,
						Line:   1,
						Column: 17,
					},
					End: Position{
						Offset: 16,
						Line:   1,
						Column: 17,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lexer := NewScanningLexer(strings.NewReader(tt.input))

			var tokens []*Token

			for {
				token := lexer.NextToken(context.Background())

				tokens = append(tokens, token)
				if token.Type == TokenTypeEOF {
					break
				}
			}

			if diff := cmp.Diff(tt.expected, tokens); diff != "" {
				t.Errorf("Tokens mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
