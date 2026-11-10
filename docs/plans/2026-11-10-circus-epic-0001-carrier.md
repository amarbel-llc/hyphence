# hyphence as the circus EPIC 0001 record carrier (2026-11-10)

Answer to hyphence#17: circus EPIC 0001 D1/D2 asks whether hyphence can carry
the signed grant (`! profile-grant-v1`), the profile transition approval
(`! profile-transition-v1`) and the capability document. The source is circus
`docs/plans/2026-10-22-d1-capability-artifact-and-grant-chain.md`, sections 2,
2a and 6. Everything below was checked against hyphence master plus this
branch's vectors. The circus side is read, not verified.

## 1. API gaps

All four asks are met by the typed head (RFC 0005, landed 2026-11-09:
`Head` in Rust and Go). Nothing has to be added for them.

- **Typed edge with a lock.** `- profile = NAME@<digest>` is RFC 0003's
  locked field. It decodes to `Field { name: "profile", value:
  Markl(MarklId { purpose: Some(NAME), digest }) }` (Go: `KindField` with a
  `*MarklID` value). `- _prev = @<digest>`, `- widens = @<digest>`,
  `- from = @…` and `- to = @…` are id-less locks (`purpose: None`). A
  digest's lexeme is checked (format runes, blech32 alphabet) but never
  decoded.
- **Canonical emission from fixed-order lines.** `Head::encode` /
  `Head.Encode` write RFC 0005's canonical bytes. Prefix order is
  `#`, `-`, `@`, `!`, and the sort is stable, so the `-` lines keep the
  caller's order. FDR 0034's line order is kept as given. Field lines are
  written `- key = value`. A value is bare when it is an Ident and quoted
  otherwise. Every line shape in D1 section 2 encodes bare except one (next
  point).
- **A locked right has a quoted purpose.** A markl purpose is bare only when it
  fits marklid's `PurposeBare` (`[a-zA-Z0-9_/-]+`). So
  `- right = roustabout:grammar:attest-seq@<digest>` reads fine but is written
  `- right = "roustabout:grammar:attest-seq"@<digest>`. An unlocked right
  (`spinclass:merge`) and the other colon- or dot-bearing values
  (`spend-usd:1.00`, RFC 3339 times) stay bare, because `:` `.` `+` are
  allowed inside an Ident and refused only at the end. Vector
  `d1-grant-run-root` pins this.
- **A repeated `- right = …` line.** Repeats are allowed. `Head::fields()`
  yields every field in order, with no merging and no dedup. The canonical
  form does not sort within a rank, so two grants with the same set of
  rights in a different order are different bytes and different digests.
  circus should fix an order for set-valued lines (FDR 0034's, or sorted)
  before it encodes, and a verifier should refuse anything else.
- **A signature line kept out of the signed bytes.** There is no dedicated
  API, and none is needed. The consumer drops the line from `head.lines`
  (Go `Head.Lines`) and encodes the rest: that encoding is the bytes to
  digest and sign. The position of the signature line does not matter,
  because it is removed before encoding and the rest sort by rank. piggy
  RFC 0013 (`piggy-act-v1`) likewise signs and digests canonical typed-head
  bytes. Suggestion, not a requirement: write the signature as a field,
  `- sig = piggy-escalation-v1@<sshsig>`, rather than the bare term
  `- piggy-escalation-v1@<sshsig>`. A bare locked term has the same shape as
  any locked tag or reference, so a verifier would be finding the signature
  by its purpose string; a named field is unambiguous. Both spellings parse.
- **Capability document.** D2's `name@digest` catalog nodes are the same
  locked edge. hyphence has no opinion on the D2 body; trellis/espalier body
  parsing is cutting-garden's.

## 2. The D1 vectors against the PEG

hyphence's grammar can only say whether a record's lines parse and how they
are spelled. Most of V-D1-1..17 are verdicts of the chain verifier or the
predicate evaluator (`onward`, expiry, subset, stacking, budgets, attestation
checks). They need spinclass's and the frontend's harnesses, and the content
grammar has nothing to say about them.

The record shapes that the vectors use were added to hyphence's shared corpus
(`go/hyphence/testdata/rfc_vectors.txt`, byte-identical in `rust/`). Each is a
`head/canonical` vector, so it is checked three ways: langlang against
`hyphence-content.peg` (`just test-grammar-vectors`), and the Rust and Go
typed heads for decode, canonical encode and idempotence:

| Vector | From | What it pins |
|---|---|---|
| `d1-grant-card-root` | V-D1-1 | all FDR 0034 lines, repeated rights, budget, signature line; unspaced input reordered and spaced |
| `d1-grant-narrowing-link` | V-D1-2 | `_prev` id-less lock, no signature line; already canonical |
| `d1-grant-run-root` | V-D1-3 | `session = -`, a locked right (quoted purpose), a quoted time re-spelled bare |
| `d1-grant-widening-root` | V-D1-9 | `widens` id-less lock |
| `d1-profile-transition` | V-D1-15 | the transition record, `from`/`to` id-less locks |

All five parse under the PEG and agree byte for byte across Rust and Go. The
values are illustrative placeholders, not real grants, keys or digests.
V-D1-12..14 (the attestation record) are not covered, because D1 does not
spell that record's lines.

**Vector format.** Use the four-column `rfc_vectors.txt` lines
(`name<TAB>input-b64<TAB>outcome<TAB>expected-b64`). `head/canonical` is for a
shape that must parse and has a canonical spelling, and `grammar/reject` is
for content that must be refused. `just debug-vector-encode` produces the
base64. circus can keep its own suite in the same format and stage the
grammar from the `.#hyphence-content-grammar` flake output. Shapes that
hyphence itself should guarantee can come here as new vectors.

## Not decided here

The field table, which lines a grant must carry, the act text and every
verdict in D1 section 3 belong to circus and spinclass. Whether to adopt the
field-form signature line is circus's call.
