# hyphence

**hyphence** (hyphen-fence) is a text-based document format that separates a
document's *metadata* from its *body* with a `---` fence, in the spirit of
front-matter but with a small, strictly specified grammar:

```
! some-type-tag
@ blake2b256-…
key value

the body bytes, verbatim, after the blank line
```

The metadata section is a sequence of single-line entries — the `!` line carries
an opaque **type** tag, the `@` line an opaque **digest** reference, and plain
`key value` lines carry arbitrary fields. The format imposes no scheme on the
type or digest payloads; it round-trips them verbatim. The body is never
buffered or interpreted — it streams through untouched.

The canonical, normative specification (MUST/SHOULD/MAY) is **RFC 0001**; every
conforming implementation must agree with the shared conformance vectors.

- [`docs/rfcs/0001-hyphence.md`](docs/rfcs/0001-hyphence.md) — normative format specification
- [`docs/man.7/hyphence.md`](docs/man.7/hyphence.md) — tutorial / reference manual

## Content grammar

The envelope treats a line's value as opaque, but the value does have a grammar:
RFC 0002 (as amended by RFC 0003), whose normative text is the langlang-validated
[`docs/rfcs/hyphence-content.peg`](docs/rfcs/hyphence-content.peg). Both
implementations ship it (`hyphence.ParseContent` in Go, `parse_content` in Rust),
and `hyphence validate` applies it.

It is layered, not folded in: `decode` stays envelope-only so documents written
against RFC 0001 alone keep decoding, and content strictness is opt-in. The
practical consequence is the charset-strict markl-id digest slot — a payload
outside the blech32 alphabet is rejected by `validate` but still decodes.

The grammar composes across repos, upstream to downstream, via langlang
`@import`: **piggy → hyphence → trellis** (cutting-garden). hyphence imports the
markl-id primitives (`String`, `Char`, `Format`, `DataChar`) from piggy's
`marklid.peg`, and exports its own productions for downstream consumers as the
`.#hyphence-content-grammar` flake output, so they stage the `.peg` hermetically
rather than vendoring a copy. Because langlang resolves `@import` relative to the
importing file, the local grammar gates stage piggy's `marklid.peg` beside the
`.peg` before running (`.#grammarStaged`).

## Layout

The repository is polyglot, with one implementation per language under a
language-named top-level directory:

```
go/      Go implementation: the hyphence library (package hyphence) + the hyphence CLI (cmd/hyphence)
rust/    Rust implementation (crate hyphence, RFC 0001 envelope; zero deps, edition 2024)
docs/    RFC 0001-0003, hyphence-content.peg (langlang-validated grammar
         companion) + the man.1 (CLI) and man.7 (format) manuals
```

The Go library is dewey-only (no madder-internal imports), so it builds
standalone. Its `Type` / `Digest` value types are opaque wrappers — they hold
the `!` tag and `@` digest as plain strings and round-trip them verbatim.

The Rust crate (`rust/hyphence/`) implements the same RFC 0001 envelope
(`Document::{decode,encode}`) with zero dependencies, and the same content
grammar (`parse_content`). A root virtual-workspace
`Cargo.toml` exists only so the crate resolves as a cargo git dependency; it
does not disturb the `go/` ↔ `rust/` peer layout. Both implementations are
checked against the same `rfc_vectors.txt` (kept byte-identical by a
`vectors-equality` flake check).

## CLI

The Go module also ships a `hyphence` command (`cmd/hyphence`) — format-only
inspection and re-emission of on-disk hyphence documents, reading a file or `-`
for stdin:

```sh
hyphence validate FILE   # check against RFC 0001 + the RFC 0002/0003 content grammar (exit non-zero on error)
hyphence meta FILE       # print just the metadata section (boundaries stripped)
hyphence body FILE       # stream just the body bytes
hyphence format FILE     # re-emit in canonical line order (idempotent)
```

`nix build` produces it at `result/bin/hyphence`; `nix run .#hyphence -- …` runs
it directly. The reference manual is [`docs/man.7/hyphence.md`](docs/man.7/hyphence.md)
(installed as `hyphence(7)`).

## Build & test

The build is Nix-driven (an igloo-based flake + gomod2nix for Go, a cargo
virtual workspace for Rust). The justfile's `default` recipe is the full
pre-merge CI lane.

```sh
just                 # build + test (the pre-merge CI gate)
just build           # regenerate gomod2nix.toml + run the flake checks
just test            # go + rust + bats suites, grammar validation (+ vectors), go vet, and the impure eng lint
just test-go         # just the Go test suite (RFC conformance needs -tags test)
just test-rust       # just the Rust test suite (unit + RFC conformance)
just test-bats       # just the CLI integration suite (zz-tests_bats)
just validate-grammar # just the langlang grammar-validation gate (docs/rfcs/hyphence-content.peg)
just test-grammar-vectors # just the langlang -input vector cross-check (RFC vectors vs. the grammar)
just codemod-fmt     # format the tree in place (nix fmt / conformist repair)
```

The hermetic test gates are flake checks: `checks.go-test` runs
`go test -tags test ./...` (the `-tags test` flag is mandatory — the Go RFC
conformance suite is behind a `//go:build test` constraint), `checks.rust-test`
runs `cargo test`, and `checks.vectors-equality` keeps the two impls' vector
files identical. `nix flake check` (via `just build`) runs all of them.

A devShell (`nix develop`, or `direnv allow`) provides the Go and Rust
toolchains, `gomod2nix`, and the conformist formatter/linter tools.
