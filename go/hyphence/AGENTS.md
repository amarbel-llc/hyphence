# hyphence

I/O for hyphence (hyphen-fence) delimited format (metadata/content separation).

## Key Types

- `MetadataWriterTo`: Writer interface with metadata content check

## Format

Uses `---` as boundary between metadata and content sections.

## Format-only document model

Slice 1 of the `hyphence` CLI added a format-only document model
that sits next to the type-aware Coder/Reader machinery and shares
the package's `Reader` boundary scanner. These types operate on
RFC 0001 syntax only — no body decoding, no type-tag dispatch.

- `Document` / `MetadataLine`: structured representation of a
  parsed metadata section. Bodies are never buffered.
- `MetadataStreamer`: passthrough metadata consumer used by
  `hyphence meta`.
- `MetadataBuilder`: parses metadata lines into a `Document` for
  `hyphence format`.
- `MetadataValidator`: strict per-line RFC 0001 checker used by
  `hyphence validate`. Its `CheckContent` field additionally runs the
  RFC 0002/0003 content-grammar pass; it is opt-in, and the field's
  own comment explains why defaulting it on would break the
  retired-spelling compatibility vectors.
- `FormatBodyEmitter`: blob consumer for `hyphence format` —
  emits canonicalized metadata then streams body bytes.
- `Canonicalize`: sort metadata per RFC §Canonical Line Order.
- Sentinels: `ErrMalformedMetadataLine`, `ErrInvalidPrefix`,
  `ErrInlineBodyWithAtReference`, `ErrMalformedContent`.

## Content grammar (RFC 0002/0003)

`content.go` implements the content grammar — the productions that
govern what a metadata line's *value* may say, as distinct from the
envelope that carries it. It is the Go half of a three-way lockstep
with `docs/rfcs/hyphence-content.peg` (normative) and
`rust/hyphence/src/content.rs`.

- `ParseContent(prefix byte, content string) error`: checks one line's
  content against the production its prefix selects. It is the typed
  parse (`parseLine`) with its result discarded, so the recognizer and
  the typed head below share one parser.
- `ContentSyntaxError`: the concrete error, carrying the prefix, a
  rune offset, and what was expected. Matches
  `errors.Is(err, ErrMalformedContent)`.

Two things to know before changing it:

- **It is deliberately NOT in the decode path.** `Reader`/`Decoder`
  stay envelope-only, because RFC 0002's scope boundary is that
  existing decoders remain conforming, and three normative vectors
  carry retired pre-RFC-0003 spellings to prove old documents still
  decode. `hyphence validate` opts in instead.
- **The digest slot is charset-strict**:
  `Digest <- Format '-' DataChar+ !IdentRune`, over `Format`/`DataChar`
  imported from piggy's `marklid.peg`. `blake2b256-9bt3` is rejected
  because `b` is outside blech32. Changing this means changing the
  `.peg`, both parsers, and the shared vectors together.

## Typed head (RFC 0005)

`head.go` is the Go twin of `rust/hyphence/src/head.rs`: the metadata
section read into typed lines, plus the canonical encoder
(`docs/rfcs/0005-canonical-emission.md`).

- `DecodeHead(r)` / `HeadFromDocument(doc)` → `Head` (`[]HeadLine` with
  leading/trailing comments); a content error is a `*HeadContentError`
  matching `ErrMalformedContent`.
- `Line` is a `LineKind`-tagged struct (description, field, term, blob,
  type); `Value` is a markl id (`*MarklID`) or text; `MarklDigest` is the
  `format-data` slot (`Digest` is taken by `type_digest.go`).
- `Head.Encode(w, body, hasBody)` / `Head.Document()` spell every line
  canonically; `Quote` is the Doddish quoter.
- The `head/canonical` vectors (`head_vectors_test.go`) pin byte equality
  with Rust, and so does `testdata/0005-vectors.txt`
  (`emission_vectors_test.go`), a copy of `docs/rfcs/0005-vectors.txt`, the
  file piggy replays. Edit the `docs/rfcs/` file and copy it to both
  `testdata/` dirs; the `vectors-equality` flake check refuses drift. `DecodeHead`'s `hasBody` comes from `Reader.HasBody()`, the
  only place an empty body and no body differ (the Blob consumer sees the
  same empty stream). `Reader`, `Decoder`, `Coder` and `CoderToTypedBlob`
  each run their own copy of the state machine; all four end EOF inside
  the metadata section with `abortUnterminatedMetadata`
  (`ErrUnterminatedMetadata`).

## See also

- `docs/man.7/hyphence.md` — tutorial / reference manual.
- `docs/rfcs/0001-hyphence.md` — normative format specification (MUST/SHOULD/MAY).
- `testdata/rfc_vectors.txt` + `rfc_conformance_test.go` — RFC conformance test harness.
