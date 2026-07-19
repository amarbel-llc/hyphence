---
status: proposed
date: 2026-07-18
authors: Sasha F (with Clown)
supersedes: RFC 0002 §Lock grammar (docs/rfcs/0002-content-grammar.md) — narrow, section-scoped
revised: 2026-07-18 (four items from Sasha's follow-up ruling: the
  reference-aliasing direction as truth-plus-forward-reference to a
  future RFC 0004, `=` spacing standardization with a per-medium
  canonicality principle, MarklId restructured as a two-slot term
  rather than an opaque lexeme, and a field-residence clarifying
  paragraph)
---

# RFC 0003 — Markl-Atomic Locks

## Status

Proposed. Supersedes **only** RFC 0002's "Lock grammar" section, and every place that section is referenced from (the `Lock` production in "Per-line-kind productions," "Field lines and reference-valued fields," the compatibility note in "`<` Deprecation," "Decoder Behavior," "Encoder Behavior," "Compatibility," and the lock-form rows of "Test Vectors"). Everything else in RFC 0002 — the deleted `/`-shape convention, field-line syntax, the `<` deprecation itself, trailing comments, the collapsed canonical order — is unaffected and remains governed by RFC 0002. RFC 0002's own `status` does not change to `superseded`; it stays `proposed` with a pointer at each affected section (see "Compatibility" below for the exact edits).

This RFC also documents (not specifies — full grammar deferred to a future RFC 0004) the truth about `<`'s shipped role as a binding operator, distinct from and unaffected by the `<` PREFIX deprecation; standardizes optional spacing around `=` with a per-medium canonicality principle; restructures the MarklId lexeme as a two-slot term instead of an opaque one; and adds one clarifying paragraph on field residence. These four items were ruled on in the same follow-up as the items below and are bundled into this RFC rather than split out, per that ruling.

Ruled by Sasha on [linenisgreat/hyphence#6](https://code.linenisgreat.com/linenisgreat/hyphence/issues/6), 2026-07-18.

## Abstract

RFC 0002 gave a locked value its own grammar: an appended `Lock = SP DigestTerm` suffix, spelled `SP "@" Ident`. This RFC retires that production. A locked value was never a hyphence-level construct in need of hyphence-level syntax — it is an ordinary **markl id**, and a markl id's own text form (`[purpose@]format-data`, madder RFC 0002 / markl-id(7) §STRUCTURE) already joins a purpose to its payload with an unspaced `@`. Trellis now recognizes that join as its own dedicated production, `MarklTerm <- (String / Ident) '@' Ident` (cutting-garden `docs/rfcs/0014-trellis.peg`), tried before `Ident`/`QuotedRef` — so "locking a value" reduces to *writing that value as a purpose-full markl id* — no separate suffix, no new hyphence-side production, just an explicit `MarklTerm` alternative added to the handful of hyphence productions that could hold one. `! md@blake2b256-…`, `- one/uno@blake2b256-…`, and `- blocks=task/other@blake2b256-…` are, each, one atomic `MarklTerm`. The wire form of a markl id carries no spaces; presentation layers may add them for readability. This is a superseding RFC, not an in-place revision of RFC 0002, because it changes what RFC 0002's already-merged lock-form test vectors demonstrate, and RFC 0001 §Test Vectors requires supersession for that.

Alongside the lock supersession, this revision documents four adjacent findings and refinements ruled on in the same follow-up: the `<` content-internal operator dodder already ships (a document-scoped binding, not a lock — direction only, full grammar in a future RFC 0004); optional, medium-dependent spacing around `=`; a corrected, structured (not opaque) MarklId production; and a one-paragraph field-residence clarification.

## Notational Conventions

The key words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) when, and only when, they appear in all capitals.

## Relationship to RFC 0001 and RFC 0002

- **No envelope changes.** Same scope boundary as RFC 0002: this RFC lives entirely inside RFC 0001's `CONTENT` production. `BOUNDARY`, `SEPARATOR`, `PREFIX`, and the six PREFIX characters are untouched.
- **Narrow supersession, not a rewrite.** The Lock supersession itself replaces one production (`Lock`) and the prose built on it; it does not revisit RFC 0002's other confirmed points. This RFC's revision additionally makes one further RFC 0002 production change (`FieldContent` gains optional `SP?` around `=`, "Canonical emission: spacing") and one purely additive documentation update (§Reserved: reference aliasing gains a pointer to this RFC's findings) — both scoped precisely below, neither reopening anything else RFC 0002 settled.
- **Why supersession and not revision.** RFC 0001 §Test Vectors: "Removing a vector requires a superseding RFC, since vectors are normative." RFC 0002's `unified-lock-type`, `unified-lock-reference`, and `deprecated-angle-still-accepted` vectors were written and documented as demonstrating *the* canonical lock spelling. Under this RFC that spelling is no longer canonical — the vectors' normative label changes even though (being envelope-level) their literal decode outcome does not. That is exactly the case the supersession rule is written for. See "Test Vectors" below for exactly what changes and what doesn't.

## The MarklId lexeme

A markl id's text form, per madder `docs/rfcs/0002-markl-id-format.md` §2 and `markl-id(7)` §STRUCTURE:

```
MarklId = (Purpose "@")? FormatData
```

`Purpose` provides semantic context for the payload (optional); `FormatData` is the blech32-encoded payload, HRP-tagged by format. This RFC does not duplicate that grammar — `Purpose` and `FormatData`'s own lexical rules are madder's to define, cited here by reference only, the same pattern RFC 0002 used for trellis's productions.

**Revised 2026-07-18: MarklId is `MarklTerm`, a structured two-slot production in trellis's own grammar — not admitted through `Ident`'s interior structure.** The first draft of this RFC treated a purpose-full markl id as admitted purely through an interior-`@` amendment to `IdentRune` — one opaque token, no internal structure visible to the grammar. That amendment was itself superseded before landing: cutting-garden `docs/rfcs/0014-trellis.peg` now defines a dedicated production instead, `IdentRune` reverting to its pre-amendment form (`@` is `Reserved` again, not identifier-interior):

```
MarklTerm <- (String / Ident) '@' Ident
```

tried in `BasicTerm` **before** `QuotedRef`/`Ident` (`BasicTerm <- Group / FieldPred / TypeTerm Sigil? / DigestTerm Sigil? / MarklTerm Sigil? / QuotedRef Sigil? / Ident Sigil? / Sigil`), so `a@b` and `"a b"@c` are recognized as markl terms outright, never as an identifier with a dangling `@` or a quoted string that happens to be followed by more content. `FieldPred`'s `Value` gained the same alternative: `Value <- MarklTerm / String / DigestTerm / Bareword`.

A dedicated production rather than an `Ident`-interior rule resolves three requirements the opaque-token approach couldn't:

1. **Quoting a purpose that needs it must not trap the digest inside the quotes.** `"my thing"@blake2b256-…` — **not** `"my thing@blake2b256-…"`. The latter buries the digest inside one string literal; only `MarklTerm`'s explicit `(String / Ident) '@' Ident` shape keeps the digest structurally outside the quotes.
2. **Presentation elision needs a grammar-level digest boundary.** Editor integrations that elide or trie-abbreviate a digest payload for display (see "Wire form vs. presentation" below) need to know grammatically where the digest starts, not infer it from an opaque token's internal bytes.
3. **A purpose containing a literal `@` is spelled by quoting the purpose slot** (`"a@b"@fmt-x`) — the join point is `MarklTerm`'s own unquoted `@`, a real production boundary, so a quoted purpose's interior `@` is never ambiguous with it.

Hyphence's own content grammar does not duplicate `MarklTerm`'s definition — it is cited by name, the same pattern RFC 0002 used for every other trellis production, and spelled out explicitly (ordered first, matching `BasicTerm`) in each hyphence production it applies to — see "Lock Grammar," below.

Term-initial `@` remains the `DigestTerm` sigil (`'@' Ident`, payload-only markl ids — `@blake2b256-…`, and the id-less `FieldRHS = DigestTerm` alternative, e.g. `_base=@blake2b256-…`); those are **unchanged by this RFC** and are not `MarklTerm` (no purpose slot, no join to disambiguate — `DigestTerm` is tried before `MarklTerm` in `BasicTerm` regardless). A purpose-full markl id like `md@blake2b256-…` or `piggy-piv_auth-v1@ssh_ecdsa_nistp256_pub-…` parses as one `MarklTerm` wherever hyphence content holds a locked value, with no hyphence-side grammar change required beyond retiring `Lock` (below). Resolution of the purpose portion (tag vs. reference vs. something else) is type-system-side, like every other identifier RFC 0002 §"The `/`-shape convention is deleted" already established — this RFC does not add a special case for purpose-full identifiers.

**Purpose grammar expansion (piggy-side, not this RFC).** The ruling accepts expanding markl's purpose grammar to admit general identifiers (e.g. object-id-shaped purposes containing `/`), tracked as [linenisgreat/piggy#219](https://code.linenisgreat.com/linenisgreat/piggy/issues/219) — hyphence's content grammar is unaffected either way, since it admits whatever `MarklTerm`/`Ident` trellis admits regardless of what shape a purpose takes.

## Lock Grammar (supersedes RFC 0002 §Lock grammar)

RFC 0002's production:

```
Lock = SP DigestTerm
     = SP "@" Ident
```

is **deleted**, along with every `Lock?` suffix built on it. A "locked" value is not a distinct grammatical shape — it is simply an ordinary `Ident`-position term whose content happens to be a `MarklTerm`. Because `MarklTerm` is a sibling production to `Ident` in trellis's `BasicTerm` (tried first — see "The MarklId lexeme," above), every hyphence production that previously admitted only `Ident`/`String` in a position that could be locked gains an explicit `MarklTerm` alternative, ordered first for the same reason. RFC 0002's "Per-line-kind productions" block amends to:

```
TypeContent  = MarklTerm / Ident              ; was: Ident Lock?
RefContent   = GroundTerm                     ; was: GroundTerm Lock?
GroundTerm   = MarklTerm / String / Ident     ; was: String / Ident
FieldRHS     = DigestTerm                     ; unchanged — id-less, payload-only
             / FieldValue                     ; was: FieldValue (SP DigestTerm)?
FieldValue   = MarklTerm / String / Bareword  ; was: String / Bareword
```

`BlobContent`, `DigestTerm`, and every other production RFC 0002 defined are otherwise **unchanged**.

Concretely, per line kind:

| Line | Unlocked | Locked (this RFC) | RFC 0002 (retired) |
|---|---|---|---|
| `!` | `! md` | `! md@blake2b256-…` | `! md @blake2b256-…` |
| `-` (reference) | `- one/uno` | `- one/uno@blake2b256-…` | `- one/uno @blake2b256-…` |
| `-` (typed edge) | `- blocks=task/other` | `- blocks=task/other@blake2b256-…` | `- blocks=task/other @blake2b256-…` |
| `-` (key material, no separate lock — the value *is* the pinned id) | — | `- piggy-piv_auth-v1@ssh_ecdsa_nistp256_pub-…` | not representable under RFC 0002 without quoting (linenisgreat/hyphence#6) |

The last row is the case that surfaced this RFC: a bare `-` line whose entire value is a purpose-full markl id used as key/auth material has no separate "unlocked" form to contrast against — the purpose-full id *is* the content, not a base value plus a suffix. RFC 0002's grammar had no legal unquoted place for it; this RFC's `RefContent = GroundTerm` (`GroundTerm` already admits any `MarklTerm`, including purpose-full ones) covers it directly, no quoting required.

**Compatibility note (supersedes RFC 0002's, verbatim replaced).** RFC 0002 made two claims about legacy spellings under one `<`-adjacent umbrella; this RFC's research found three distinct roles for `<` that must not be conflated, only one of which either RFC actually retires:

- **`<` as a PREFIX character** ("explicit object reference," RFC 0001 §Metadata Lines) — deprecated to a synonym of `-` by RFC 0002 §`<` Deprecation. Unaffected by this RFC.
- **`<` as a hypothetical content-internal lock separator** (`- value < markl-id`, RFC 0001's original informal table entry) — retired by RFC 0002, and this RFC does not revive it. Dodder-implementation research found no evidence this exact shape (bare value, `<`, bare digest, no name being bound to anything) was ever the real wire form; see Open Questions for what RFC 0001's table was likely actually describing.
- **`<` as the content-internal binding operator** (`name < target[@lock]`) — **this one is real, ships today, and is unaffected by any retirement in either RFC.** See "Reference aliasing: truth and direction" below; the earlier draft of this RFC conflated this role with the one above and incorrectly implied it was retired.

The two spellings this RFC's Lock supersession does change:

- The pre-RFC-0002 at-join spelling (`! type@markl-id`, RFC 0001's informal table) — which RFC 0002 called illegal content-grammar and required rejecting — is legal again under this RFC, because it is the *same spelling* this RFC now canonicalizes for a different reason (markl-atomicity, not RFC 0001's original informal convention). A decoder implementing this RFC's content grammar accepts it.
- RFC 0002's spaced `SP @digest` spelling (`! md @blake2b256-…`) is now the retired one. A decoder implementing this RFC's content grammar SHOULD treat it as malformed; a decoder implementing only RFC 0001 envelope validation is unaffected either way (content stays opaque at that layer).

## Wire form vs. presentation

This section is about the markl id's **internal** purpose-join spacing specifically. For the separate question of spacing around the `=` and `<` *operators* in hyphence content, see "Canonical emission: spacing" below — the two are independent and must not be conflated.

**Normative:** the wire form of a markl id's purpose-join carries no spaces, in every position and every medium. `Lock`'s deletion is not merely "spaces became optional" — there is no suffix left to space or not space; the purpose-join `@` is interior to a single `MarklTerm` token, and identifiers (per trellis's `Reserved`/`SP1` exclusion) cannot contain whitespace at all. This does not vary by medium the way operator spacing does (below): a markl id is the same bytes everywhere it appears.

**Non-normative:** presentation layers (rendered views, editor buffers, documentation) MAY insert visual spacing around the purpose-join, and MAY elide or trie-abbreviate the format-data payload for display — this is a rendering choice with no bearing on the wire bytes, the same relationship RFC 0002's `-`/`<` deprecation has to canonical vs. non-canonical *emission* (a presentation layer is not an encoder). Confirmed example already in use: pandoc-markdown-style digest concealing in editor integrations.

## Canonical emission: spacing

`FieldContent` gains optional whitespace around `=`, matching trellis's own `FieldPred <- FieldName SP? FieldOp SP? (ValueList / Value)`:

```
FieldContent = FieldName SP? "=" SP? FieldRHS    ; was: FieldName "=" FieldRHS
```

The `<` binding operator (see "Reference aliasing: truth and direction," below) is likewise spaced, matching the form dodder already ships (`writeReferencedObjects` emits `"- %s < %s"`, `go/internal/echo/object_metadata_fmt_hyphence/formatter_components.go:200`).

Both spellings — spaced and unspaced — parse under the amended grammar; the question this section answers is which one a **conforming encoder** should prefer. The answer differs by medium, extending the same per-medium canonicality principle RFC 0002 §Canonical order across media already established for line ordering (hyphence type-last vs. box type-early, both canonical for their own medium):

- **Hyphence metadata lines** (this RFC, this document's medium): canonical emission is **spaced** — `- key = value`, `- name < target`. Hyphence is a single-statement-per-line medium; there is no compactness pressure, and spacing matches the binding form dodder already ships.
- **Box format / espalier interiors** (trellis RFC 0014, dodder box(7)): canonical emission remains **unspaced** — space is trellis's own atom/term separator (`Step <- Term (SP !Combinator Term)*`), so an unspaced operator is not a stylistic choice there but a grammatical necessity; spacing `key = value` inside a box interior would be indistinguishable from two ANDed terms.
- **Markl ids, in either medium:** always space-free — see "Wire form vs. presentation," above. This is the one spacing rule that does not vary by medium, because it is not an operator between terms; it is content interior to one term.

## Dodder implementation status

Read-only research against the `dodder` repository (no changes made), to ground rather than assume the "partially markl-atomic" recollection that motivated checking this before drafting:

- **The type-lock and reference-lock encoder have always emitted the atomic form**, never RFC 0002's spaced form. `writeTypeAndSig` emits `"! %s@%s"` (`go/internal/echo/object_metadata_fmt_hyphence/formatter_components.go:171-178`); `writeReferencedObjects` emits `"- %s@%s"` for a locked reference (same file, lines 181-220).
- **The decoder matches**: `readType` parses `!type` or `!type@sig` through piggy's `markl.Lock` coder (`go/internal/echo/object_metadata_fmt_hyphence/text_parser2.go:234-255`, especially 247-252); `readReference` parses `ref` or `ref@lock` the same way (same file, lines 257-314, especially 282-304).
- **Consequence**: dodder never implemented RFC 0002's spaced Lock spelling. This RFC's atomic canonicalization is not a migration target for dodder — it is dodder's status quo, now formalized at the hyphence spec layer for the first time. No dodder-side compatibility work is implied by this RFC landing.
- **`<` inside a `-` line's content is dodder's shipping binding operator**, not aliasing in the sense RFC 0002 §Reserved: reference aliasing meant (a short local name for a reference), and distinct from the deprecated `<` PREFIX character RFC 0002 addresses. See "Reference aliasing: truth and direction," below.

## Reference aliasing: truth and direction

RFC 0002 §Reserved: reference aliasing calls alias syntax "reserved, unspecified" — a forward reference RFC 0001 left open, deferred to a future RFC. Research for this RFC found that a form of it already ships. Ruled on: this is a **binding**, not a lock and not RFC 0002's originally-imagined "short local name" aliasing — a distinct mechanism with its own semantics. This section states the confirmed truth and direction; the full grammar and remaining design are deferred to a future **RFC 0004**, not specified here.

**The shipped form.** A `-` line's content-internal `" < "` (space, `<`, space) binds a name to a target, optionally locked: `name < target[@markl-id]`. Confirmed in dodder:

- Encoder: `writeReferencedObjects` emits `"- %s < %s"` / `"- %s < %s@%s"` (name, target, optional lock) — `go/internal/echo/object_metadata_fmt_hyphence/formatter_components.go:200,202`; `writeBlobReferences` emits the same shape for a bound blob reference, `"- %s < @%s"` — same file, line 241.
- Decoder: `ReadFrom`'s `OpTagSeparator` case rewrites `" < "` to `" = "` before dispatch (`go/internal/echo/object_metadata_fmt_hyphence/text_parser2.go`, ~lines 64-69) — a field-shaped rewrite, not a bespoke alias parser: a binding is field-shaped internally (`name = target`) even though it is spelled with `<` on the wire. `readBlobReference` mirrors it for the blob case (same file, lines 149-158).

**Semantics (ruled, not derived — direction for RFC 0004 to formalize):**

- A binding is a **persisted, document-scoped lock table** — its original purpose is a metadata-defined lockfile: pin a name to a specific target for the lifetime of the document.
- A bound name is usable in metadata value positions **and in the body** — body use is **type-mediated**: the binding table is available to body interpreters. For `!md`, the binding table **is** a markdown link-reference-definition table — `[fred]` reflinks in the body resolve through it.
- Bare names **shadow lexically at use sites** — a bare identifier that matches a binding resolves to the binding at that use site, ordinary lexical shadowing.
- **Purpose-full (pinned) identifiers never consult the binding table** — the shadow escape. Writing a full `MarklTerm` directly bypasses the table entirely; it is already resolved.
- Binding a name that **also** resolves globally (a real tag or object id) is a **validation warning**, not an error — flags the shadowing ambiguity for the author.
- Table semantics are **whole-table, order-free** — consistent with RFC 0001's "decoders MUST accept metadata lines in any order; line order MUST NOT affect semantics." The table is a flat name→target map once parsed; source order of binding lines carries no semantic weight.
- **Canonical placement**: bindings sort first among `-` lines — presentation only, per the order-free rule above; this refines rather than contradicts RFC 0002 §`<` Deprecation's collapsed `-` bucket.
- **Not queryable** — bindings are a hyphence-document-internal indirection; they do not surface as trellis query-layer fields.

Grammar productions, the exact validation-warning trigger, and anything not listed above are **not specified by this RFC** — RFC 0004's scope.

## Field residence

A metadata field line (`- key=value`) and a type-owned field of the same key name that might also appear inside the object's **body** (a `!task` object's TOML body declaring a field with the same name) are not automatically the same field — which layer a given field "lives in" is the consuming type system's concern (tracked in [linenisgreat/dodder#377](https://code.linenisgreat.com/linenisgreat/dodder/issues/377): type-declared body-resident vs. metadata-resident fields, a unified index, loud bidirectional-collision detection), not hyphence's; this RFC (and RFC 0002) define line grammar only, with no cross-layer field-namespace policy of their own.

## Open Questions

1. **The `< markl-id` legacy spelling's provenance.**
   **Resolved**: RFC 0001's original informal table entry (`- value < markl-id`) was very likely an early, imprecise description of the binding operator documented above (`name < target`, described loosely as "a lock" when it is a binding declaration), rather than a distinct, now-vanished lock-join spelling. Dodder-implementation research found no evidence of a bare lock-join use of `<` (value, `<`, digest, no name being bound) distinct from the binding form.

## Test Vectors

New vectors are **appended**, per RFC 0001 §Test Vectors' convention; per that section, superseding an existing vector's normative meaning (not removing it — the base64 rows still exist and still decode the same way) requires exactly the kind of superseding RFC this is. The file stays byte-identical across `go/hyphence/testdata/rfc_vectors.txt` and `rust/hyphence/testdata/rfc_vectors.txt`.

**What changes for the three existing RFC-0002-era lock vectors.** `unified-lock-type`, `unified-lock-reference`, and `deprecated-angle-still-accepted` all use the now-retired spaced `SP @digest` spelling. Their `OUTCOME` column is **unchanged** — content is opaque at the envelope layer these vectors test (`legacy/parse-ok` / `document/parse-ok`), so the spaced spelling still decodes fine and always will, per RFC 0001 §Versioning's indefinite-support principle. What changes is only their *documentation*: they no longer demonstrate "the canonical lock spelling" (RFC 0002's framing) — they demonstrate a **retired, envelope-still-valid** spelling (this RFC's framing). `id-less-field-lock` and `field-predicate-line`/`trailing-comment-quoting-composes` are **unaffected** — the id-less form (`_base=@digest`) is explicitly unchanged by this RFC, and the latter two never involved a lock.

**New vectors**, demonstrating the atomic canonical spelling (all envelope-level, same pattern as RFC 0002's — content is opaque to both harnesses regardless of content-grammar canonicity):

| Name | Outcome | Demonstrates |
|---|---|---|
| `atomic-lock-type` | `legacy/parse-ok` | `! md@blake2b256-abc` — canonical type-lock spelling (this RFC) |
| `atomic-lock-reference` | `document/parse-ok` | `- one/uno@blake2b256-def` — canonical reference-lock spelling |
| `atomic-lock-typed-edge` | `document/parse-ok` | `- blocks=task/other@blake2b256-ghi` — canonical typed-edge lock spelling |
| `atomic-key-material-papi-shape` | `document/parse-ok` | `- piggy-piv_auth-v1@ssh_ecdsa_nistp256_pub-qqxyz` — the bare-value key-material shape that surfaced this RFC (linenisgreat/hyphence#6), verbatim per cutting-garden's own `0014-trellis.peg` conformance vector for the same identifier |
| `quoted-purpose-marklterm` | `document/parse-ok` | `- "my thing"@blake2b256-xyz` — `MarklTerm`'s quoted-purpose-slot form; the digest sits outside the quotes, per "The MarklId lexeme" |
| `spaced-field-emission` | `document/parse-ok` | `- due = "2026-08-01"` — the canonical *spaced* field-operator emission this RFC's revision standardizes for hyphence lines, per "Canonical emission: spacing" |

## See Also

- `docs/rfcs/hyphence-content.peg` — the formal, langlang-validated companion grammar extracting this RFC's (and RFC 0002's) productions ([linenisgreat/hyphence#7](https://code.linenisgreat.com/linenisgreat/hyphence/issues/7)); validate with `just validate-grammar`.
- `docs/rfcs/0002-content-grammar.md` — the RFC this document narrowly supersedes (Lock grammar section only).
- `docs/rfcs/0001-hyphence.md` — the envelope RFC, unaffected.
- madder `docs/rfcs/0002-markl-id-format.md` and `markl-id(7)` §STRUCTURE / §PURPOSE IDS — the normative MarklId lexeme this RFC cites by reference.
- cutting-garden `docs/rfcs/0014-trellis.peg` — the `MarklTerm` production (and `BasicTerm`/`Value` ordering) this RFC's design depends on.
- [linenisgreat/hyphence#6](https://code.linenisgreat.com/linenisgreat/hyphence/issues/6) — the issue and ruling this RFC implements.
- [linenisgreat/piggy#219](https://code.linenisgreat.com/linenisgreat/piggy/issues/219) — the purpose-grammar expansion this RFC's design depends on being accepted (piggy-side, not blocking).
- [linenisgreat/dodder#377](https://code.linenisgreat.com/linenisgreat/dodder/issues/377) — the field-residence design issue "Field residence" points to.
- A future **RFC 0004** — the full binding grammar "Reference aliasing: truth and direction" defers to; does not exist as of this RFC.
- [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) — normative-language conventions.
