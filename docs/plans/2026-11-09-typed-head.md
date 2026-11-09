# Typed head and canonical encoder (2026-11-09)

**Goal:** hyphence owns the typed reading and canonical writing of a document's
head, in Rust and Go, so consumers add only their own field tables on top.
Operator ruling 2026-11-09; first consumer piggy RFC 0013 (signed acts), slice
S2a of piggy `docs/plans/2026-11-09-rfc-0013-acts-sequencing.md`.

**Normative text:** [RFC 0005](../rfcs/0005-canonical-emission.md).

## API (Rust, `rust/hyphence/src/head.rs`)

```rust
pub struct Digest { /* format, data: text, lexeme-checked */ }
impl Digest { fn new(format, data) -> Result<Digest, InvalidDigest>; fn parse(&str) -> ...; fn format(&self); fn data(&self) }
pub struct MarklId { pub purpose: Option<String>, pub digest: Digest }   // purpose unescaped; None = `@fmt-data`
pub enum Value { Markl(MarklId), Text(String) }                         // bare word == quoted string
pub enum Line {
    Description(String),
    Field { name: String, value: Value },
    Term(Value),                                // tag or reference; `<` reads as `-`
    Blob(String),
    Type { name: String, lock: Option<Digest> },
}
pub struct HeadLine { pub line: Line, pub leading_comments: Vec<String>, pub trailing_comment: Option<String> }
pub struct Head { pub lines: Vec<HeadLine>, pub trailing_comments: Vec<String> }

Head::from_document(&Document) -> Result<Head, HeadError>
Head::decode(&[u8]) -> Result<(Head, Option<Vec<u8>>), HeadError>
Head::to_document(&self, Option<Vec<u8>>) -> Result<Document, EncodeError>
Head::encode(&self, Option<&[u8]>) -> Result<Vec<u8>, EncodeError>
Head::fields(), Head::type_line(), quote(&str)
```

The Go twin (`go/hyphence/head.go`) mirrors it with a `Kind`-tagged `Line`
struct.

## Decisions

- **One parser.** The content recognizer (`parse_content` / `ParseContent`)
  became the typed parse with its result discarded, so the reader and the
  recognizer cannot disagree.
- **Bare word and quoted string are one kind** (`Value::Text`). They are
  content-equal, so the encoder picks the spelling.
- **Markl ids stay text.** The lexeme is checked (format runes, blech32
  alphabet); blech32 is never decoded. The crate stays zero-dependency.
- **`<` folds into `-`** on read (RFC 0002 synonym) and is never written. The
  source prefix is not kept: piggy refuses bare term lines anyway, so only an
  error message would lose it.
- **Comments are kept** by the canonical encoder (RFC 0001, RFC 0002); a
  consumer drops them on the model first.
- **Order within a rank is the caller's**: the encoder sorts by prefix rank,
  stably. A consumer orders fields by permuting `Head::lines`.
- **Purpose bareness uses marklid's `PurposeBare`**, the markl canonical
  spelling, not hyphence's wider `Ident`. Confirmed with piggy.
- **Escapes** are piggy-markl `quote_purpose`'s full set, so a LF in a value is
  written `\n`. Confirmed with piggy.
- **Encode is fallible**: an unlocked non-Ident type name, a purpose-less markl
  id as a term, and LF in free text have no spelling.

## Out of scope

Markl decoding, body parsing (trellis/espalier is cutting-garden's), consumer
field tables and refusals, RFC 0004 bindings, any change to RFC 0001. No
grammar amendment was needed.

## Follow-ups

- piggy-pigpen's minimal framing (`crates/piggy-pigpen/src/hyphence.rs`) can
  move onto this crate (piggy RFC 0008 Compatibility); piggy's call.
- `hyphence format` still re-emits through the envelope-only path, so it does
  not respell values. Whether it should switch to the typed encoder (which
  refuses retired spellings) is open.
