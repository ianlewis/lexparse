# lexparse

[![GoDoc](https://godoc.org/github.com/ianlewis/lexparse?status.svg)](https://godoc.org/github.com/ianlewis/lexparse)
[![Go Report Card](https://goreportcard.com/badge/github.com/ianlewis/lexparse)](https://goreportcard.com/report/github.com/ianlewis/lexparse)
[![tests](https://github.com/ianlewis/lexparse/actions/workflows/pre-submit.units.yml/badge.svg)](https://github.com/ianlewis/lexparse/actions/workflows/pre-submit.units.yml)
[![codecov](https://codecov.io/gh/ianlewis/lexparse/graph/badge.svg?token=PD7UEVGU5S)](https://codecov.io/gh/ianlewis/lexparse)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-%23FE5196?logo=conventionalcommits&logoColor=white)](https://conventionalcommits.org)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/ianlewis/lexparse/badge)](https://api.securityscorecards.dev/projects/github.com/ianlewis/lexparse)

Experimental lexer/parser library written in Go. Currently under active development.

---

`lexparse` aims to provide a simple API for writing lexers and parsers. The API is based loosely off of Rob Pike's [Lexical Scanning in Go](https://www.youtube.com/watch?v=HxaD_trXwRE) where the lexer's state is itself a function. `lexparse` extends this concept to the parser and also implements state via an interface to allow state to hold data without the need for a closure.

## Installation

You can install the library into your project using `go get`.

```shell
go get github.com/ianlewis/lexparse
```

## Lexer

The `Lexer` operates on a stream of text and outputs lexemes or tokens. It is a finite state machine where each state includes some logic for parsing the data while in that state. The `Lexer` maintains a cursor to the start of the currently processed lexeme in addition to the underlying reader's position. When the lexeme has been fully processed it can be emitted to a channel for further processing by the `Parser`.

Each state can be implemented as a function which returns the next state. The example below generates word lexemes separated by whitespace.

```go
func lexWord(_ context.Context, l *lexparse.Lexer) (lexparse.LexState, error) {
	// Find the next whitespace.
	ws, err := l.Find([]string{" ", "\t", "\n"})
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

    // Emit non-empty tokens.
	if l.Width() > 0 {
		l.Emit(wordType)
	}

    // Discard the whitespace.
	if _, err := l.Discard(len(ws)); err != nil {
		return nil, err
	}

	return lexparse.LexStateFn(lexWord), err
}
```

## Parser

The `Parser` operates on a stream of lexemes (e.g. tokens) from the `Lexer` and creates an [abstract syntax tree](https://en.wikipedia.org/wiki/Abstract_syntax_tree). It is also a finite state machine where each state includes logic for processing lexemes and adding tree nodes.
