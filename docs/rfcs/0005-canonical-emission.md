---
status: proposed
date: 2026-11-09
authors: Sasha F (with Clown)
---

# RFC 0005 — Canonical Emission

## Status

Proposed. Will move to `accepted` upon merge of this RFC.

## Abstract

RFC 0001 fixes the envelope an encoder writes, RFC 0002 the order of the metadata lines, and RFC 0003 the spacing around a field line's `=`. None of them says how a line's *value* is spelled: when a string is quoted, how it is escaped, how a markl id's purpose slot is written. Several spellings parse to the same content (`- state=open`, `- state = "open"`, `- state='open'`), so two conforming encoders can write different bytes for equal content. This RFC defines the one **canonical spelling** of a hyphence head, so that equal typed content always encodes to equal bytes. A consumer that signs or digests a document (piggy RFC 0013's signed acts are the first) needs exactly that.

It adds no grammar. Every canonical spelling is already admitted by `docs/rfcs/hyphence-content.peg`; this RFC picks one among the admitted spellings.

## Notational Conventions

The key words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) when, and only when, they appear in all capitals.

## Relationship to RFC 0001–0003

- **Extends, does not supersede.** RFC 0001's envelope rules, RFC 0002's line order and `<` deprecation, and RFC 0003's spacing all stand. This RFC adds rules where they are silent.
- **Optional for encoders in general.** An encoder that only round-trips (RFC 0001 §Conformance) need not be canonical. An encoder that claims to emit **canonical form** MUST follow every rule below.
- **No envelope change, no content-grammar change.** Decoders are unaffected.

## Typed content

Canonical form is defined over a document's **typed content**: what the content grammar parses, with every quoted string unescaped. Two documents have equal typed content when, line for line, they parse to equal values:

| Line | Typed content |
|---|---|
| `#` | the free text, verbatim |
| `-`/`<` field (`FieldContent`) | the field name (unescaped) and the value |
| `-`/`<` term (`RefContent`) | the value |
| `@` | the blob reference text (unescaped) |
| `!` | the type name, and the lock digest if any |
| `%` line | the text after `% `, verbatim, entangled with the next non-comment line (RFC 0001) |
| trailing comment | the text after `%`, verbatim (RFC 0002) |

A **value** is one of:

- a **text**: what `Bareword`, `Ident` or `String` parsed to, with the `String` unescaped. A bare word and a quoted string carrying the same characters are the SAME value: `open` and `"open"` are equal.
- a **markl id**: an optional purpose (unescaped when it was quoted) and a digest, `format-data`, taken as text (hyphence never decodes the payload). A field value written `@format-data` (RFC 0002 `DigestTerm`) is a markl id with no purpose.

`<` and `-` lines have equal typed content when their content is equal (RFC 0002: `<` is a synonym of `-`).

## Unescaping

A `String` (`"…"` or `'…'`) is unescaped by the Doddish rules piggy-markl applies to a quoted purpose: `\n`, `\t`, `\r`, `\a`, `\b`, `\f`, `\v` stand for LF, TAB, CR, BEL, BS, FF and VT; a backslash before any other character stands for that character (so `\"` is `"`, `\\` is `\`, and `\q` is `q`).

## Canonical spelling

A canonical encoder MUST write, for each line:

1. **Prefix.** `-` for every field and term line; `<` MUST NOT be emitted (RFC 0002).
2. **Field lines** as `- NAME = VALUE`: one space either side of `=` (RFC 0003 §Canonical emission: spacing).
3. **A text** (a field name, a text value, a term, a blob reference) **bare** when the whole text is an `Ident` under `hyphence-content.peg` (non-empty, every character an `IdentRune` under the strict sigil rule), and otherwise **quoted**.
4. **Quoted** means double quotes, with `\` written `\\`, `"` written `\"`, LF `\n`, TAB `\t`, CR `\r`, BEL `\a`, BS `\b`, FF `\f`, VT `\v`, and every other character written as itself. Single quotes MUST NOT be emitted. A value containing a line feed is therefore written `\n` and never breaks the line.
5. **A markl id with a purpose** as `PURPOSE@format-data`, with no spaces (RFC 0003 §Wire form). The purpose is **bare** when it matches piggy `marklid.peg`'s `PurposeBare` (`[a-zA-Z0-9_/-]+`, piggy RFC 0011 §2.1), and otherwise quoted as in rule 4; the join `@` follows the closing quote. This is markl's own canonical purpose spelling, which is narrower than `Ident`: `caldav:x@blake2b256-…` is written `"caldav:x"@blake2b256-…`.
6. **A markl id with no purpose** (only legal as a field value) as `@format-data`.
7. **The type line** as `! NAME` when unlocked, or `! PURPOSE@format-data` when locked, the name spelled as a purpose by rule 5. An unlocked type name that is not an `Ident` has no canonical spelling (`TypeContent` admits no quoted unlocked name), and MUST be refused by the encoder.
8. **`#` lines** with their text verbatim.
9. **Comments are kept**, as RFC 0001 (`%` lines are preserved on round-trip) and RFC 0002 (content-preserving encoders MUST keep trailing comments) require: each `%` line written `% TEXT` immediately before the line it is entangled with, document-tail `%` lines after the last line, and a trailing comment written after the line's content as one space, `%`, and its text verbatim. A consumer whose canonical form drops comments removes them from the typed content before encoding.

A description, comment or trailing-comment text containing a line feed has no spelling and MUST be refused by the encoder.

## Canonical order

Lines are emitted in RFC 0002's collapsed order: `#`, then `-`, then `@`, then `!`. The sort is **stable**: within one rank, lines keep the order the encoder is given them in. RFC 0002 says the `-` bucket is in source order; a canonical encoder applies that to its **input order**, so a consumer that needs a particular order among its own lines (a field table, sorted set values) puts them in that order before encoding. Hyphence imposes no order of its own within a rank.

## Envelope

As RFC 0001 §Encoder Behavior: `---\n`, the lines, `---\n`, then — only when a body is present — one `\n` and the body bytes, unchanged. An empty body (zero bytes) is still a body and still gets the separator; a consumer that treats it as absent passes no body.

## The guarantee

Two documents with equal typed content, lines in the same order within each rank, equal comments, and equal bodies MUST encode to equal bytes. Canonical form is a fixed point: decoding canonical bytes and encoding them again MUST reproduce them.

## Out of scope

- Markl payload decoding, checksums and lengths (piggy-markl's).
- Any type's field table: which fields exist, their order, set semantics, refusals (the consumer's).
- The body's internal format (the `!` type's).
- RFC 0004 bindings (`name < target`), which the content grammar does not admit yet.

## Implementations

Both reference implementations ship a typed head and this encoder: Rust `Head` (`rust/hyphence/src/head.rs`; `Head::decode`, `Head::from_document`, `Head::to_document`, `Head::encode`) and Go `Head` (`go/hyphence/head.go`). They share one parser with the content recognizer, so a line the recognizer accepts is a line the typed reader reads.

## Test Vectors

Appended to the shared corpus under a new outcome, `head/canonical`: the input decodes into a typed head, encodes to exactly the expected field, and the expected field re-encodes to itself. Both harnesses (Rust `head_canonical_vectors`, Go `TestHeadCanonicalVectors`) run every one, and the content-grammar harnesses check every input line.

| Name | Demonstrates |
|---|---|
| `head-canonical-value-kinds` | bare word, quoted string, purpose-full markl id, id-less `@digest`, a quoted value that can be bare written bare |
| `head-canonical-quoted-field-name` | a field name that needs quotes keeps them; one that does not loses them |
| `head-canonical-locked-type-and-reference` | a locked type and a locked reference unchanged; type line moved last |
| `head-canonical-purpose-spelling` | `"a@b"@…` (join is the second `@`), `"md"@…` written bare, `caldav:x@…` quoted by `PurposeBare` |
| `head-canonical-unspaced-to-spaced` | `key="v"` written `key = v`; `<` written `-` |
| `head-canonical-comments-kept` | `%` lines, a glued trailing comment and a document-tail comment kept |
| `head-canonical-escapes` | `\"`, `\\`, `\t` round-trip; `\n` stays two characters; an unknown escape collapses |
| `head-canonical-body` | the body follows the separator unchanged |

## See Also

- `docs/rfcs/0001-hyphence.md`, `docs/rfcs/0002-content-grammar.md`, `docs/rfcs/0003-markl-atomic-locks.md`, `docs/rfcs/hyphence-content.peg`.
- piggy `docs/rfcs/0011-markl-id-format.md` §2 (`PurposeBare`, the quoted purpose) and piggy-markl `spell_purpose`/`quote_purpose`, whose escape set rule 4 matches.
- piggy `docs/rfcs/0013-signed-hyphence-acts.md` §3.1, the first consumer: its canonical form is this RFC's applied after piggy's own normalizations.
- `docs/plans/2026-11-09-typed-head.md` — the implementation plan for this slice.
