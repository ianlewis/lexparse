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

// Package lexparse defines a set of interfaces that can be used to define
// generic lexers and parsers over byte streams.
package lexparse

import (
	"context"
	"errors"
)

// LexParse lexes the content starting at initState and passes the results to a
// parser starting at initFn. The resulting root node of the parse tree is returned.
func LexParse[V comparable](
	ctx context.Context,
	r BufferedRuneReader,
	initState State,
	initFn ParseFn[V],
) (*Node[V], error) {
	l := NewLexer(r, initState)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p := NewParser[V](l.Lex(ctx))
	n, pErr := p.Parse(ctx, initFn)
	cancel()

	<-l.Done()

	// Check for lexing error.
	var err error
	lErr := l.Err()
	if lErr != nil && !errors.Is(lErr, context.Canceled) {
		err = lErr
	}

	// If no lexing error return parsing error.
	if err == nil {
		err = pErr
	}

	return n, err
}
