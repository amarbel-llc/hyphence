---
status: proposed
date: 2026-07-18
authors: Sasha F (with Clown)
supersedes: RFC 0002 §Lock grammar (docs/rfcs/0002-content-grammar.md) — narrow, section-scoped
---

# RFC 0003 — Markl-Atomic Locks

## Status

Proposed. Supersedes **only** RFC 0002's "Lock grammar" section, and every place that section is referenced from (the `Lock` production in "Per-line-kind productions," "Field lines and reference-valued fields," the compatibility note in "`<` Deprecation," "Decoder Behavior," "Encoder Behavior," "Compatibility," and the lock-form rows of "Test Vectors"). Everything else in RFC 0002 — the deleted `/`-shape convention, field-line syntax, the `<` deprecation itself, trailing comments, the collapsed canonical order, the reserved aliasing slot — is unaffected and remains governed by RFC 0002. RFC 0002's own `status` does not change to `superseded`; it stays `proposed` with a pointer at each affected section (see "Compatibility" below for the exact edits).

Ruled by Sasha on [linenisgreat/hyphence#6](https://code.linenisgreat.com/linenisgreat/hyphence/issues/6), 2026-07-18.

## Abstract

RFC 0002 gave a locked value its own grammar: an appended `Lock = SP DigestTerm` suffix, spelled `SP "@" Ident`. This RFC retires that production. A locked value was never a hyphence-level construct in need of hyphence-level syntax — it is an ordinary **markl id**, and a markl id's own text form (`[purpose@]format-data`, madder RFC 0002 / markl-id(7) §STRUCTURE) already joins a purpose to its payload with an unspaced `@`. Once trellis's `Ident` production admits that join as ordinary identifier content (cutting-garden `docs/rfcs/0014-trellis.peg`, the interior-`@` amendment this RFC's design depends on), "locking a value" reduces to *writing that value as a purpose-full markl id* — no separate suffix, no new production. `! md@blake2b256-…`, `- one/uno@blake2b256-…`, and `- blocks=task/other@blake2b256-…` are, each, one atomic `Ident`/`FieldValue`. The wire form carries no spaces; presentation layers may add them for readability. This is a superseding RFC, not an in-place revision of RFC 0002, because it changes what RFC 0002's already-merged lock-form test vectors demonstrate, and RFC 0001 §Test Vectors requires supersession for that.

## Notational Conventions

The key words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** in this document are to be interpreted as described in [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) when, and only when, they appear in all capitals.

## Relationship to RFC 0001 and RFC 0002

- **No envelope changes.** Same scope boundary as RFC 0002: this RFC lives entirely inside RFC 0001's `CONTENT` production. `BOUNDARY`, `SEPARATOR`, `PREFIX`, and the six PREFIX characters are untouched.
- **Narrow supersession, not a rewrite.** This RFC replaces one production (`Lock`) and the prose built on it. It does not revisit RFC 0002's other confirmed points.
- **Why supersession and not revision.** RFC 0001 §Test Vectors: "Removing a vector requires a superseding RFC, since vectors are normative." RFC 0002's `unified-lock-type`, `unified-lock-reference`, and `deprecated-angle-still-accepted` vectors were written and documented as demonstrating *the* canonical lock spelling. Under this RFC that spelling is no longer canonical — the vectors' normative label changes even though (being envelope-level) their literal decode outcome does not. That is exactly the case the supersession rule is written for. See "Test Vectors" below for exactly what changes and what doesn't.

## The MarklId lexeme

A markl id's text form, per madder `docs/rfcs/0002-markl-id-format.md` §2 and `markl-id(7)` §STRUCTURE:

```
MarklId = (Purpose "@")? FormatData
```

`Purpose` provides semantic context for the payload (optional); `FormatData` is the blech32-encoded payload, HRP-tagged by format. This RFC does not duplicate that grammar — `Purpose` and `FormatData`'s own lexical rules are madder's to define, cited here by reference only, the same pattern RFC 0002 used for trellis's productions.

Hyphence does not need its own `MarklId` production to admit this shape as content. Trellis's `Ident` production (cited by RFC 0002 §Lexical foundation, unchanged) was amended in cutting-garden `docs/rfcs/0014-trellis.peg` to make `@` identifier-interior under the same positional rule already used for sigils:

```
IdentRune <- '-' / '/' / (SigilRune &IdentRune) / ('@' &IdentRune) / (!Reserved !SP1 .)
```

Term-initial `@` remains the `DigestTerm` sigil (`'@' Ident`, payload-only markl ids — `@blake2b256-…`, and the id-less `FieldRHS = DigestTerm` alternative, e.g. `_base=@blake2b256-…`); those are **unchanged by this RFC**. `@` that is *not* term-initial — i.e. following an identifier rune — is now ordinary identifier content. A purpose-full markl id like `md@blake2b256-…` or `piggy-piv_auth-v1@ssh_ecdsa_nistp256_pub-…` therefore already parses as one `Ident` wherever hyphence content cites `Ident`, with no hyphence-side grammar change required beyond retiring `Lock` (below). Resolution of the purpose portion (tag vs. reference vs. something else) is type-system-side, like every other identifier RFC 0002 §"The `/`-shape convention is deleted" already established — this RFC does not add a special case for purpose-full identifiers.

**Purpose grammar expansion (piggy-side, not this RFC).** The ruling accepts expanding markl's purpose grammar to admit general identifiers (e.g. object-id-shaped purposes containing `/`), tracked as a piggy issue, not specified here — hyphence's content grammar is unaffected either way, since it admits whatever `Ident` trellis admits regardless of what shape a purpose takes.

## Lock Grammar (supersedes RFC 0002 §Lock grammar)

RFC 0002's production:

```
Lock = SP DigestTerm
     = SP "@" Ident
```

is **deleted**, along with every `Lock?` suffix built on it. A "locked" value is not a distinct grammatical shape — it is simply an ordinary `Ident` / `FieldValue` whose content happens to be a purpose-full `MarklId`. RFC 0002's "Per-line-kind productions" block amends to:

```
TypeContent  = Ident                    ; was: Ident Lock?
RefContent   = GroundTerm                ; was: GroundTerm Lock?
FieldRHS     = DigestTerm                ; unchanged — id-less, payload-only
             / FieldValue                ; was: FieldValue (SP DigestTerm)?
```

`GroundTerm`, `FieldValue`, `DigestTerm`, and every other production RFC 0002 defined are otherwise **unchanged** — this RFC touches only the three lines above.

Concretely, per line kind:

| Line | Unlocked | Locked (this RFC) | RFC 0002 (retired) |
|---|---|---|---|
| `!` | `! md` | `! md@blake2b256-…` | `! md @blake2b256-…` |
| `-` (reference) | `- one/uno` | `- one/uno@blake2b256-…` | `- one/uno @blake2b256-…` |
| `-` (typed edge) | `- blocks=task/other` | `- blocks=task/other@blake2b256-…` | `- blocks=task/other @blake2b256-…` |
| `-` (key material, no separate lock — the value *is* the pinned id) | — | `- piggy-piv_auth-v1@ssh_ecdsa_nistp256_pub-…` | not representable under RFC 0002 without quoting (linenisgreat/hyphence#6) |

The last row is the case that surfaced this RFC: a bare `-` line whose entire value is a purpose-full markl id used as key/auth material has no separate "unlocked" form to contrast against — the purpose-full id *is* the content, not a base value plus a suffix. RFC 0002's grammar had no legal unquoted place for it; this RFC's `RefContent = GroundTerm` (`GroundTerm` already admits any `Ident`, including purpose-full ones) covers it directly, no quoting required.

**Compatibility note (supersedes RFC 0002's, verbatim replaced).** Both of RFC 0002's claims about legacy spellings are revised:

- The pre-RFC-0002 at-join spelling (`! type@markl-id`, RFC 0001's informal table) — which RFC 0002 called illegal content-grammar and required rejecting — is legal again under this RFC, because it is the *same spelling* this RFC now canonicalizes for a different reason (markl-atomicity, not RFC 0001's original informal convention). A decoder implementing this RFC's content grammar accepts it.
- RFC 0002's spaced `SP @digest` spelling (`! md @blake2b256-…`) is now the retired one. A decoder implementing this RFC's content grammar SHOULD treat it as malformed; a decoder implementing only RFC 0001 envelope validation is unaffected either way (content stays opaque at that layer).
- The pre-RFC-0002 angle-join spelling (`- value < markl-id`, lock via content-internal `<`) remains retired, as RFC 0002 already established — this RFC does not revive it. (Dodder-implementation research for this RFC found no evidence `<` was ever used this way in practice — see "Dodder implementation status" below; RFC 0001's original table description of it may have been describing a spelling that was never the real wire form.)

## Wire form vs. presentation

**Normative:** the wire form of a locked value carries no spaces. `Lock`'s deletion is not merely "spaces became optional" — there is no suffix left to space or not space; the purpose-join `@` is interior to a single `Ident`/`FieldValue` token, and `Ident` (per trellis's `Reserved`/`SP1` exclusion) cannot contain whitespace at all.

**Non-normative:** presentation layers (rendered views, editor buffers, documentation) MAY insert visual spacing around the purpose-join, and MAY elide or trie-abbreviate the format-data payload for display — this is a rendering choice with no bearing on the wire bytes, the same relationship RFC 0002's `-`/`<` deprecation has to canonical vs. non-canonical *emission* (a presentation layer is not an encoder). Confirmed example already in use: pandoc-markdown-style digest concealing in editor integrations.

## Dodder implementation status

Read-only research against the `dodder` repository (no changes made), to ground rather than assume the "partially markl-atomic" recollection that motivated checking this before drafting:

- **The type-lock and reference-lock encoder have always emitted the atomic form**, never RFC 0002's spaced form. `writeTypeAndSig` emits `"! %s@%s"` (`go/internal/echo/object_metadata_fmt_hyphence/formatter_components.go:171-178`); `writeReferencedObjects` emits `"- %s@%s"` for a locked reference (same file, lines 181-220).
- **The decoder matches**: `readType` parses `!type` or `!type@sig` through piggy's `markl.Lock` coder (`go/internal/echo/object_metadata_fmt_hyphence/text_parser2.go:234-255`, especially 247-252); `readReference` parses `ref` or `ref@lock` the same way (same file, lines 257-314, especially 282-304).
- **Consequence**: dodder never implemented RFC 0002's spaced Lock spelling. This RFC's atomic canonicalization is not a migration target for dodder — it is dodder's status quo, now formalized at the hyphence spec layer for the first time. No dodder-side compatibility work is implied by this RFC landing.
- **Not confirmed, and out of scope for this RFC to resolve**: dodder's reference/blob-reference lines also use a content-internal `" < "` (space-less-than-space) separator for aliasing (`alias < target[@lock]`), distinct from the deprecated `<` PREFIX character RFC 0002 addresses. This directly touches RFC 0002 §Reserved: reference aliasing's "no grammar is defined here" — there already is a shipping one. Whether to formalize it is under discussion as of this RFC's drafting; not resolved here. See **Open Questions**.

## Open Questions

1. **Reference aliasing may already have a real answer, found adjacent to this RFC's research but out of its assigned scope.** Dodder's `-`/blob-reference lines use `" < "` (space-dash... space-less-than-space) as a content-internal alias separator (`go/internal/echo/object_metadata_fmt_hyphence/formatter_components.go:200,202,241`; decoder-side `text_parser2.go` `ReadFrom`'s `OpTagSeparator` case, ~lines 64-69, and `readBlobReference`, lines 149-158). RFC 0002 §Reserved: reference aliasing states "No grammar is defined here" and treats it as future work — but a grammar already exists and ships. This RFC was scoped narrowly to the lock supersession and does not formalize aliasing; whether it belongs in a future RFC, in an amendment to this one, or stays deferred is not decided here. Flagged rather than resolved, per the same discipline that produced this RFC's own design.
2. **The `< markl-id` legacy spelling's provenance.** RFC 0001's original informal table described a lock spelling of `- value < markl-id`. Dodder-implementation research found no evidence this exact shape (bare value, `<`, bare digest — no alias) was ever the real wire form; the only `<`-involving pattern found is the alias separator (`alias < target[@lock]`), a different shape. Whether RFC 0001's original table was describing an aspirational spelling that was never implemented, or a real historical spelling this research didn't surface, is not confirmed.

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

## See Also

- `docs/rfcs/0002-content-grammar.md` — the RFC this document narrowly supersedes (Lock grammar section only).
- `docs/rfcs/0001-hyphence.md` — the envelope RFC, unaffected.
- madder `docs/rfcs/0002-markl-id-format.md` and `markl-id(7)` §STRUCTURE / §PURPOSE IDS — the normative MarklId lexeme this RFC cites by reference.
- cutting-garden `docs/rfcs/0014-trellis.peg` — the interior-`@` `IdentRune` amendment this RFC's design depends on.
- [linenisgreat/hyphence#6](https://code.linenisgreat.com/linenisgreat/hyphence/issues/6) — the issue and ruling this RFC implements.
- [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) — normative-language conventions.
