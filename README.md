# videre

`videre` is a fast, modal terminal editor with a vi-first workflow and minimal runtime dependencies.

It is built for keyboard-driven editing, quick startup, and a clean full-screen terminal UI.  
The current codebase is a Rust rewrite focused on performance, safety, and functional parity with the original Go/C implementations.

## Why `videre`

- Modal editing that stays close to muscle memory
- Tight terminal feedback with a low-friction UI
- Single-binary workflow with simple build/run/install paths
- Practical feature set: motions, visual selections, search, marks, yoinks, paste, and command mode
- High memory stability and low latency via Rust's zero-cost abstractions

## Features

- Modes: Normal, Insert, Visual, Visual Line, and `:` command entry
- Navigation: `hjkl`, arrows, word motions, paragraph motions, `%`, `gg`, `G`, `{n}G`
- Editing: `i a I A o O`, `x`, `d`, `y`, `c` (including operator+motion forms and count prefixes)
- Search: `/`, `n`, `N`, `f/F/t/T`, `;`, `,`
- Commands: `:w`, `:q`, `:q!`, `:qa!`, `:wq`, `:e <file>`, `:{number}`, `:help`
- Extras: clipboard integration, syntax highlighting (tree-sitter), Git status line, stable memory profile

## Build

For a release build with optimizations:

```sh
cargo build --release
```

The binary will be located at `target/release/videre`.

## Run

```sh
cargo run -- path/to/file
```

## Install

```sh
cargo install --path .
```

## Benchmarking

Performance is measured using Rust's standard benchmarking infrastructure or custom integration tests:

```sh
cargo bench
```

## Project Layout

- `src/main.rs`: Application entrypoint and lifecycle
- `src/editor.rs`: Core editor state and text manipulation logic
- `src/input.rs`: Modal key handling and command dispatch
- `src/ui.rs`: Terminal rendering and screen management
- `src/syntax.rs`: Syntax highlighting integration (tree-sitter)
- `videre.1`: man page

## Notes

- Linux/macOS terminals are the primary target.
- On crash/hard-kill, restore terminal state with `reset` or `stty sane`.

## License

GPLv3. See `LICENSE`.
