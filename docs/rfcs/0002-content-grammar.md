---
status: proposed
date: 2026-07-18
authors: Sasha F (with Clown)
---

# RFC 0002 — Content Grammar

## Status

Proposed. Will move to `accepted` upon merge of this RFC.

## Abstract

RFC 0001 specifies hyphence's envelope — boundaries, metadata-line prefixes, and body separation — and leaves each metadata line's `CONTENT` deliberately opaque ("arbitrary UTF-8 except LF"). This RFC specifies that content: a grammar for what may appear after each `PREFIX SP`, defined by reuse of the lexical and per-term productions of trellis (cutting-garden RFC 0014, `0014-trellis-query-language.md` + `0014-trellis.peg`), the query/data language dodder and cutting-garden are converging on. It deletes the `/`-shape convention for distinguishing tags from references, unifies the two existing lock spellings into one appended `@digest` form, gives fields a syntax with no new PREFIX character, and defines reference-valued fields as typed edges. It does not touch the envelope: every RFC 0001-conforming decoder remains conforming, because content-grammar validation is an additional, optional layer on top of RFC 0001's opaque-content model.

## Notational Conventions

The key words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) when, and only when, they appear in all capitals.

## Relationship to RFC 0001

RFC 0001 §Versioning anticipates this document: "Future format additions or revisions MUST be tracked in subsequent RFCs... that supersede or extend this one." This RFC **extends** RFC 0001; it does not supersede it.

Scope boundary:

- **No envelope changes.** `BOUNDARY`, `SEPARATOR`, `PREFIX`, and the six PREFIX characters (`!` `@` `#` `-` `<` `%`) are unchanged. This RFC does not add, remove, or reassign a PREFIX character — see RFC 0001 §Conformance "Wire-format extensions" for why that would require different treatment.
- **Content only.** Everything specified here lives inside RFC 0001's `CONTENT` production — the bytes between `PREFIX SP` and the line's terminating `LF`.
- **Existing decoders remain conforming.** An RFC 0001 decoder that never inspects content beyond "not LF" is unaffected by this RFC and needs no changes to stay conforming. Content-grammar validation, as specified below, is an additional layer a decoder MAY implement.

## Content Grammar

### Lexical foundation

Line content is not a bespoke grammar. It reuses trellis's lexical and per-term productions directly — one lexical universe, per the confirmed design. This RFC does not duplicate those productions; it cites them by name from `0014-trellis.peg` and layers hyphence-specific composition rules (line-kind dispatch, the lock suffix) on top.

Productions reused verbatim from `0014-trellis.peg`:

| Production | Role |
|---|---|
| `Ident`, `IdentRune`, `Reserved` | opaque identifier lexis (rune-class exclusion; `-` and `/` admitted inline; the strict term-final sigil rule) |
| `String`, `Char` | quoted literal (the `QuotedRef` escape hatch for content containing reserved runes) |
| `DigestTerm` | `'@' Ident` — blob-identity ground term |
| `FieldName` | `String / Ident` |
| `Bareword` | `IdentRune+` — unquoted field value |
| `SP`, `SP1` | mandatory whitespace |

Hyphence line content is restricted to the **ground fragment** of trellis — what RFC 0014 §Isometry calls **espalier**: a data instance is "a query that matches exactly itself" (`=`-semantics only, no other operators, no value lists, no sigils, no closures). A metadata line asserts a fact about one object; it is not a query. Consequently, in content position:

- **MUST NOT** use non-`=` field operators (`!=` `*=` `^=` `$=` `<` `<=` `>` `>=` `~=`). A field line's `FieldOp` is always `=`.
- **MUST NOT** use sigil suffixes (`:` `+` `.` `?`), negation (`^`), value lists (`[v1, v2]`), OR-groups, or combinators (`->` `<-` etc.). None of these have a meaning for a single ground fact.
- **MUST NOT** use `TypeTerm` (`!Ident`) inside content — the hyphence `!` PREFIX already occupies that role at the line level; re-using trellis's own `!` sigil inside content would be redundant and is disallowed.

These restrictions mean hyphence content productions below are a strict subset of trellis's `BasicTerm` / `FieldPred`, not the full grammar — implementers should not feed a raw content string to a general trellis parser expecting query semantics.

### Per-line-kind productions

Informal notation, matching RFC 0001's style (not PEG); production names in `PascalCase` not listed here are trellis productions cited above.

```
CONTENT[!]  = TypeContent
CONTENT[@]  = BlobContent
CONTENT[#]  = free text                 ; unchanged from RFC 0001
CONTENT[-]  = DashContent
CONTENT[<]  = DashContent               ; deprecated synonym, see "`<` Deprecation"
CONTENT[%]  = opaque                    ; unchanged from RFC 0001

TypeContent  = Ident Lock?
BlobContent  = String / Ident           ; markl-id, or a file-path (quoted if it
                                         ; contains spaces or reserved runes)
DashContent  = FieldContent / RefContent
RefContent   = GroundTerm Lock?
GroundTerm   = String / Ident           ; QuotedRef or bare Ident
FieldContent = FieldName "=" FieldRHS
FieldRHS     = DigestTerm                     ; id-less: purely content-addressed target
             / FieldValue (SP DigestTerm)?    ; normal: value, optionally locked
FieldValue   = String / Bareword
Lock         = SP DigestTerm
```

`DashContent` is ordered `FieldContent / RefContent`: a line whose content contains an unescaped `=` between a `FieldName` and a value is a field line; otherwise it is a reference/tag line. This is the same "syntactically distinct... under the trellis grammar" test point 4 of the originating issue describes — `FieldPred` is tried before the bare-identifier alternatives in trellis's own `BasicTerm`, and hyphence inherits that ordering.

`FieldRHS`'s `DigestTerm` alternative (the id-less form, e.g. `_base=@blake2b256-…`) depends on a trellis-side grammar change that has not landed as of this writing: `0014-trellis.peg`'s `Value <- String / Bareword` does not admit `DigestTerm` today. Confirmed 2026-07-18: trellis's `Value` production will be extended upstream (RFC 0014) to add a `DigestTerm` alternative, rather than hyphence defining a local, trellis-incompatible extension — see **Open Questions** for how this was surfaced and resolved. Until that upstream change lands, `FieldRHS = DigestTerm` here is forward-declared against it.

### Lock grammar

RFC 0001 had two inconsistent, informally-specified lock spellings:

- `! type@markl-id` — at-join, no space, `!` lines only.
- `- value < markl-id` — angle-join, content-internal `<` token (distinct from the `<` PREFIX character), `-` lines only.

Both are replaced by one production, used uniformly across every line kind that carries a lock:

```
Lock = SP DigestTerm
     = SP "@" Ident
```

Concretely: `! md @blake2b256-…`, `- one/uno @blake2b256-…`, `- blocks=task/other @blake2b256-…`. The digest spelling is pinned as `@`-prefixed, matching box format, trellis's own `DigestTerm`, and madder URIs.

**Compatibility note.** Neither legacy spelling parses under the productions above: `type@markl-id` is not a legal `Ident` (`@` is a `Reserved` rune, excluded from `IdentRune`), and the content-internal `<` token has no production at all in this grammar. A decoder that implements content-grammar validation per this RFC MUST reject both legacy spellings as malformed content. A decoder that implements only RFC 0001 envelope validation is unaffected — content remains opaque at that layer, so documents using either legacy spelling remain RFC 0001-envelope-conforming. Encoders conforming to this RFC MUST emit only the unified `SP @digest` spelling.

### Field lines and reference-valued fields (typed edges)

A field line (`- key=value`) is a ground field predicate: `FieldName "=" FieldValue`, closing the isometry gap with box format's `key=value` interior syntax. No new PREFIX character is introduced — fields ride the existing `-` (and deprecated `<`) content grammar, disambiguated from bare references by the presence of `FieldName "="`.

`FieldName`'s trellis production (`String / Ident`) permits a *quoted* field name — the same escape hatch trellis itself uses for names containing reserved runes (`"event.summary"*="standup"` is a trellis RFC 0014 conformance vector). This RFC inherits that unmodified, so a quoted field name is legal hyphence content too: `- "event.summary"="standup"`. Confirmed intended (2026-07-18), not merely an incidental consequence of grammar reuse.

A **reference-valued field with a lock** is a typed edge:

```
- blocks=task/other @blake2b256-…
```

`blocks` is the edge label (the field name), `task/other` is the target identifier, and `@blake2b256-…` pins the edge to a specific content digest of the target at write time. See "Semantic Translation" below for dodder resolution-time meaning.

The **id-less form** omits the target identifier entirely, for purely content-addressed targets with no separate object identity:

```
- _base=@blake2b256-…
```

This is the `FieldRHS = DigestTerm` alternative above — the `@digest` sits directly where `FieldValue` would otherwise go, with no separating `SP` (contrast the locked form, where `SP DigestTerm` follows a `FieldValue`). As noted in "Per-line-kind productions," this alternative is forward-declared against an upstream trellis grammar extension (`Value` gaining a `DigestTerm` alternative) that had not landed as of this RFC's drafting — see **Open Questions**.

### The `/`-shape convention is deleted

RFC 0001's informal rule — "bare values are tags; values containing `/` are object references" — is deleted. It was designed when object ids were transparent and syntactically required `/`; dodder's direction (dodder FDR 0018) and trellis make identifiers opaque, with tag-vs-reference resolved through the type system at consumption, never from token shape. A `-` line's `GroundTerm` content (when not a `FieldContent`) is simply an opaque `Ident` or `String` — `/` carries no grammatical significance. See "Semantic Translation" for the resolution-time behavior this enables.

## `<` Deprecation

The `<` PREFIX character ("explicit object reference," RFC 0001 §Metadata Lines) is **deprecated to a synonym of `-`**. Its original function was to override the `/`-shape convention when a bare `-` line's shape-based inference was wrong; the shape convention no longer exists (previous section), so `<` has no remaining function distinct from `-`.

- Decoders **MUST** continue accepting `<` per RFC 0001 §Decoder Behavior (this RFC does not touch PREFIX acceptance).
- Encoders conforming to this RFC **MUST NOT** emit `<`; all reference, tag, and field lines **MUST** be emitted with `-`.
- `<` line content grammar is identical to `-` line content grammar (`DashContent`, above).

**Side effect on canonical order.** RFC 0001 §Encoder Behavior's canonical order lists three separate buckets for `-`/`<`-shaped content: (2) locked object references, (3) aliased object references, (4) bare object references and tags. That three-way rank existed to model the locked/aliased/bare distinction referenced by issue #128. Under this RFC, lock-ness is a suffix property of any `-` line (any `DashContent` MAY carry a `Lock`), not a distinct line shape or PREFIX choice — so the distinction no longer needs modeling as a separate rank. Buckets (2)–(4) collapse into a single bucket: all `-` lines (bare references, locked references, field predicates, locked field predicates alike), in source order. RFC 0001's canonical order becomes:

1. Description lines (`#`)
2. Reference, tag, and field lines (`-`), source order
3. Blob line (`@`)
4. Type line (`!`)

## Reserved: reference aliasing

RFC 0001 §Encoder Behavior names an "aliased object references" bucket without ever defining alias syntax — a forward reference RFC 0001 left open. This RFC formalizes that as an explicitly **reserved, unspecified** slot: a future RFC MAY define alias syntax for reference-valued content (per RFC 0001 §Conformance "Wire-format extensions," such a definition would itself need to be a superseding or extending RFC if it changes accepted content shapes). No grammar is defined here. Per the canonical-order collapse above, whenever aliasing is specified, an aliased line remains an ordinary `-` line and does not need its own canonical-order bucket.

This slot is deferred by the same consumer design (cutting-garden's organize-upstreaming) that motivates the rest of this RFC; it is not expected to be filled by RFC 0002.

## Canonical order across media

Hyphence and box format both have canonical orderings, and they are **both canonical** — for their own medium, not interchangeably:

- **Hyphence** (this RFC, and RFC 0001 §Encoder Behavior): type-**last** — `! type` is the final metadata line.
- **Box format** (dodder box(7)/`organize-text(7)`, trellis RFC 0014 §Isometry): type-**early** — a box line opens `[id !type ...]`, type immediately follows the id.

This is not a discrepancy to reconcile. Hyphence is a document-shaped serialization (metadata section, optional body) where the type line trailing the fields it governs reads naturally; box format is a single-line, position-sensitive listing shape where leading with the type aids scanning. Trellis's own ground fragment (espalier) is defined against box-format order — `[story-8841 !newsblur-story-v1 year=2026 ...]` — and this RFC's reuse of trellis's *term-level* lexical productions does not imply hyphence adopts box's *line-level* ordering, or vice versa. A converter between the two media reorders; it does not "fix" either one.

## Semantic Translation

What each line kind means at dodder type/object/blob resolution time. This section is descriptive (dodder resolution behavior), not itself part of the wire grammar above.

| Line content shape | Resolution-time meaning |
|---|---|
| `! Ident` | object type identifier; resolves to a type definition object |
| `! Ident Lock` | as above, pinned to a specific type-definition blob digest |
| `@ BlobContent` | blob reference (markl-id or file-path, quoted per `BlobContent = String / Ident` when it contains spaces or reserved runes) — content-addressed pointer to the object's own blob |
| `- Ident` / `- String` (GroundTerm, no `=`) | opaque identifier; **tag-vs-reference resolves through the type system at consumption**, not from `/` or any other token shape. An identifier the type system cannot resolve to an object **degrades to tag-as-string** — matching dodder's existing unmaterialized-tag behavior. |
| `- GroundTerm Lock` | as above, additionally pinned: the resolved reference is locked to the content digest of the referenced object as of write time |
| `- FieldName=FieldValue` | scalar field predicate on the current object. `FieldName` resolves via the type-defined field index (dodder FDR 0017). |
| `- FieldName=FieldValue Lock` | **typed edge**: `FieldName` is the edge label (indexable via the same type-defined field index — FDR 0017's index makes reverse traversal an index inversion), `FieldValue` is the target identifier, `Lock` pins the edge to the target's content digest at write time |
| `- FieldName=Lock` (id-less) | typed edge to a purely content-addressed target with no separate object identity |
| `_`-prefixed `FieldName` | **framework-reserved edge**, not user-declarable (mirrors trellis's `_`-prefixed reserved virtual fields). Confirmed reserved names: `_base` (organize base pinning) and `_mother` (the existing predecessor-signature edge). |
| `<` (any content) | identical resolution to the equivalent `-` line (deprecated synonym) |
| `#` | free text description; unaffected by this RFC |
| `%` | opaque comment; unaffected by this RFC |

A `Lock` on a field line implies the field's value is a reference — i.e. `- FieldName=FieldValue Lock` is always a typed edge, never a locked scalar. `- due="2026-08-01" @blake2b256-…` (locking a plainly-scalar value like a date) is grammatically well-formed per the productions above but its resolution-time meaning is **undefined by this RFC**, reserved for a future RFC to specify (or reject) should a use case arise.

## Decoder Behavior (amendments to RFC 0001)

A decoder that additionally validates content grammar per this RFC:

- **MUST** parse each line's content per the per-line-kind productions above, after RFC 0001 envelope/PREFIX validation.
- **MUST** treat content that does not match its line kind's production as a content-grammar error, distinct from RFC 0001's envelope-level errors (`ErrInvalidPrefix`, `ErrMalformedMetadataLine`, etc.).
- **MUST** accept `<` content-grammar-equivalently to `-` (see "`<` Deprecation").
- **MUST NOT** require content-grammar validation to satisfy RFC 0001 conformance — this layer is additional, per "Relationship to RFC 0001."
- An identifier that fails type-system resolution **MUST NOT** be treated as a decode error at this layer; resolution failure is a semantic (tag-degradation) outcome, not a grammar violation (see "Semantic Translation").

## Encoder Behavior (amendments to RFC 0001)

A conforming encoder:

- **MUST NOT** emit the legacy at-join type lock (`! type@markl-id`) or the legacy angle-join reference lock (`- value < markl-id`); **MUST** emit only the unified `Lock = SP DigestTerm` spelling.
- **MUST NOT** emit `<` for any line kind; **MUST** emit `-` for all reference, tag, and field lines.
- **MUST** follow the collapsed canonical order in "`<` Deprecation" above, superseding RFC 0001 §Encoder Behavior's six-bucket list for the buckets it renumbers.

## Compatibility

- Documents written under RFC 0001 alone (opaque content, either legacy lock spelling, `<` lines) remain RFC 0001-envelope-conforming indefinitely — RFC 0001 §Versioning's "decoders MUST retain support for older type strings indefinitely" principle extends here to older content spellings at the envelope layer.
- Documents are **not** guaranteed content-grammar-conforming under this RFC unless they use the unified lock spelling and (if applicable) `-` in place of `<`.
- No migration deadline is set by this RFC; encoders adopt the new spelling at their own pace, same as RFC 0001's type-string evolution model.

## Open Questions

These were ambiguities or contradictions surfaced while writing the productions above — not part of the six confirmed design points from issue #2, so flagged here rather than resolved silently. Both were put to Sasha and confirmed 2026-07-18; recorded below for provenance (original problem statement, then disposition).

1. **The id-less field form is not valid trellis `FieldPred` syntax.** `FieldRHS = DigestTerm` (e.g. `_base=@blake2b256-…`) is defined above by giving hyphence's `FieldContent` a second alternative alongside `FieldValue (SP DigestTerm)?`. But trellis's own `FieldPred <- FieldName SP? FieldOp SP? (ValueList / Value)` has `Value <- String / Bareword`, and `Bareword <- IdentRune+` excludes `@` (a `Reserved` rune) — so `@blake2b256-…` cannot appear as a trellis `Value` under the grammar as written in `0014-trellis.peg`. This means a hyphence line using the id-less form was **not** parseable as a valid trellis ground term under the isometry claim ("every hyphence line is also a valid trellis literal") that motivates point 1 of the originating issue.
   **Resolved:** trellis fixes `Value` upstream — RFC 0014's `Value` production gains a `DigestTerm` alternative, rather than hyphence defining a local, trellis-incompatible extension. This RFC's `FieldRHS = DigestTerm` alternative is therefore forward-declared against that upstream change (see "Per-line-kind productions" and "Field lines and reference-valued fields"); it is not a hyphence-local grammar deviation. Tracked upstream as [linenisgreat/cutting-garden#144](https://code.linenisgreat.com/linenisgreat/cutting-garden/issues/144).
2. **Field-line `FieldName` quoting scope.** Trellis's `FieldName <- String / Ident` permits a *quoted* field name (`"event.summary"*="standup"` in the trellis conformance vectors) as an escape hatch for names containing reserved runes. This RFC's `FieldContent` production inherits `FieldName` unmodified, so quoted field names are implicitly legal in hyphence `-` lines too (e.g. `- "event.summary"="standup"`). The originating issue's six points don't mention this case one way or the other.
   **Resolved:** intended, not an accident of grammar reuse — documented explicitly in "Field lines and reference-valued fields."

## Test Vectors

New vectors are **appended** to the existing canonical vector file, per RFC 0001 §Test Vectors' convention (`NAME \t INPUT-B64 \t OUTCOME \t EXPECTED-B64`); per that same section, removing a vector requires a superseding RFC, so this RFC only adds. The file is kept byte-identical across `go/hyphence/testdata/rfc_vectors.txt` and `rust/hyphence/testdata/rfc_vectors.txt` by the `vectors-equality` flake check; both were updated identically.

Five vectors are appended, exercising the two envelope-level outcomes capable of testing RFC 0002's new content spellings at the envelope layer (content itself remains opaque to both — these prove RFC 0001-envelope-conformance, not content-grammar validity):

| Name | Outcome | Demonstrates |
|---|---|---|
| `unified-lock-type` | `legacy/parse-ok` | `! md @blake2b256-abc` — new type-lock spelling (space-separated, `@`-joined) |
| `unified-lock-reference` | `document/parse-ok` | `- one/uno @blake2b256-def` — new reference-lock spelling |
| `field-predicate-line` | `document/parse-ok` | `- due="2026-08-01"` — a field line, quoted value, no lock |
| `deprecated-angle-still-accepted` | `document/parse-ok` | `< blocks=other/task @blake2b256-ghi` — deprecated `<` PREFIX still envelope-decodes, carrying a locked field/typed-edge line |
| `id-less-field-lock` | `document/parse-ok` | `- _base=@blake2b256-jkl` — the id-less typed-edge form |

`legacy/parse-ok`'s decoder (`TypedMetadataCoderDefault`, `go/hyphence/coder_metadata.go`) only wires the `!` and `@` prefixes, so it can't exercise the `-`/`<` forms — those needed the six-prefix decoder instead. `document/*` (the `Reader` + `MetadataValidator` harness in `go/hyphence/document_test.go`) already accepts all six prefixes but had no `parse-ok` outcome; a `document/parse-ok` case (decode via `Reader`+`MetadataValidator`, assert success) was added alongside these vectors.

On the Rust side, `rust/hyphence/src/conformance.rs`'s single `rfc_test_vectors` harness turned out not to need the unrecognized-namespace skip path this RFC originally called for: `Document::decode` (`rust/hyphence/src/decode.rs`) is a single unified decoder that already validates all six prefixes in one pass (unlike Go's legacy/document split), so `document/parse-ok` is directly implementable there too — a real match arm (`result.unwrap_or_else(|e| panic!(...))`), not a skip. Both Go and Rust genuinely decode-and-assert-success on all five new vectors; nothing is silently skipped. Implemented in [linenisgreat/hyphence#3](https://code.linenisgreat.com/linenisgreat/hyphence/issues/3).

## See Also

- `docs/rfcs/0001-hyphence.md` — the envelope RFC this document extends.
- `docs/man.7/hyphence.md` — tutorial / reference manual (not yet updated for this RFC's content grammar; tracked as [linenisgreat/hyphence#4](https://code.linenisgreat.com/linenisgreat/hyphence/issues/4)).
- cutting-garden `docs/rfcs/0014-trellis-query-language.md` and `docs/rfcs/0014-trellis.peg` — the normative grammar this RFC's productions are specified against.
- [linenisgreat/hyphence#2](https://code.linenisgreat.com/linenisgreat/hyphence/issues/2) — the issue this RFC implements (six-point proposal, confirmed by Sasha in a grill session).
- dodder FDR 0017 ("field index"), FDR 0018 ("Genre as a Type-Defined Field") — cited by the Semantic Translation section.
- [linenisgreat/hyphence#128](https://code.linenisgreat.com/linenisgreat/hyphence/issues/128) — the locked/aliased/bare rank distinction this RFC's canonical-order collapse resolves.
- [linenisgreat/cutting-garden#144](https://code.linenisgreat.com/linenisgreat/cutting-garden/issues/144) — upstream tracking issue for the trellis RFC 0014 `Value` extension the id-less field-lock form depends on (Open Questions #1).
- [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) — normative-language conventions.
