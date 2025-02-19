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
	"sync"
)

// LexParse lexes the content starting at initState and passes the results to a
// parser starting at initFn. The resulting root node of the parse tree is returned.
func LexParse[V comparable](
	ctx context.Context,
	l *Lexer,
	p *Parser[V],
) (*Node[V], error) {
	var root *Node[V]
	var lexErr error
	var parseErr error
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg.Add(1)
	go func() {
		lexErr = l.Lex(ctx)
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		root, parseErr = p.Parse(ctx)
		cancel() // Indicate that parsing is done.
		wg.Done()
	}()

	wg.Wait()

	err := lexErr
	// Do not report context.Canceled errors from the Lexer. If the context is
	// canceled by the caller the parser will also return this error.
	if err == nil || errors.Is(err, context.Canceled) {
		err = parseErr
	}

	return root, err
}
