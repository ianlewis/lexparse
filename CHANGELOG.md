# Changelog

All notable changes will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

- The EOF token emitted by the `Lexer` now includes the position where the EOF
  is encountered.
- The `Parser` now includes `NewNode` and `SetRoot` methods. These methods are
  useful for parsers that need to build the parse tree themselves instead of
  using the default methods on the `Parser`.

## [0.1.0] - 2025-02-24

- Initial release

[0.1.0]: https://github.com/ianlewis/lexparse/releases/tag/v0.1.0
