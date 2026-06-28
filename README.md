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

## Layout

The repository is polyglot, with one implementation per language under a
language-named top-level directory:

```
go/      Go implementation (module github.com/amarbel-llc/hyphence/go, package hyphence)
rust/    Rust implementation (forthcoming)
docs/    RFC 0001 + the man.7 manual
```

The Go library is dewey-only (no madder-internal imports), so it builds
standalone. Its `Type` / `Digest` value types are opaque wrappers — they hold
the `!` tag and `@` digest as plain strings and round-trip them verbatim.

## Build & test

The build is Nix-driven (an igloo-based flake + gomod2nix). The justfile's
`default` recipe is the full pre-merge CI lane.

```sh
just                 # build + test (the pre-merge CI gate)
just build           # regenerate gomod2nix.toml + run the flake checks
just test            # go test -tags test, go vet, and the impure eng lint
just test-go         # just the Go test suite (RFC conformance needs -tags test)
just codemod-fmt     # format the tree in place (nix fmt / conformist repair)
```

The hermetic test gate is the `checks.go-test` flake check, which runs
`go test -tags test ./...` — the `-tags test` flag is mandatory because the RFC
conformance suite is behind a `//go:build test` constraint.

A devShell (`nix develop`, or `direnv allow`) provides the Go toolchain,
`gomod2nix`, and the conformist formatter/linter tools.
