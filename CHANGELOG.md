# Changelog

All notable changes will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

- Renamed `CustomLexerContext` to `LexCursor` and separated it from the
  `context.Context` type.
- Renamed `ParserContext` to `ParseCursor` and separated it from the
  `context.Context` type.

## [0.3.0] - 2026-01-25

- Refactored `CustomLexer` to add a new `CustomLexerContext` type that is passed
  to state functions. This context provides access to the underlying `Lexer` as
  well as additional helper methods for lexing.
- Refactored `Parser` to add a new `ParserContext` type that is passed to state
  functions. This context provides access to the underlying `Parser` as well as
  additional helper methods for parsing.
- Updated the `CustomLexer` to not return `io.EOF` from `CustomLexer.Err`. It
  should only return unexpected errors
  ([#170](https://github.com/ianlewis/lexparse/issues/170)).
- Fixed an issue where the `Filename` field was not set on `Position` values
  returned by the `CustomLexer` and `ScanningLexer`
  ([#169](https://github.com/ianlewis/lexparse/issues/169)).

## [0.2.0] - 2025-10-31

- The EOF token emitted by the `Lexer` now includes the position where the EOF
  is encountered.
- The `Parser` now includes `NewNode` and `SetRoot` methods. These methods are
  useful for parsers that need to build the parse tree themselves instead of
  using the default methods on the `Parser`.
- The `Lexer` API has been completely redesigned. A `Lexer` interface was added
  and there are now two implementations: `ScanningLexer` and `CustomLexer`.
  `ScanningLexer` is based on `text/scanner` and can handle many use cases.
  `CustomLexer` allows for more custom lexing behavior to be implemented
  ([#129](https://github.com/ianlewis/lexparse/issues/129)).

## [0.1.0] - 2025-02-24

- Initial release

[0.1.0]: https://github.com/ianlewis/lexparse/releases/tag/v0.1.0
[0.2.0]: https://github.com/ianlewis/lexparse/releases/tag/v0.2.0
[0.3.0]: https://github.com/ianlewis/lexparse/releases/tag/v0.3.0
