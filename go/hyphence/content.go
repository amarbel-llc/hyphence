package hyphence

import (
	"errors"
	"fmt"
	"strings"
)

// This file is the Go implementation of docs/rfcs/hyphence-content.peg —
// the content grammar RFC 0002 defines and RFC 0003 amends. It is a
// RECOGNIZER: it decides whether a metadata line's content is well-formed
// under the production its PREFIX selects, and reports where and why when
// it is not. It deliberately does not build a syntax tree; no caller needs
// one yet, and an unused AST would be two more surfaces (Go and Rust) to
// keep in lockstep for nothing. Adding one later is additive.
//
// SCOPE — this is NOT wired into the decode path. Reader/Decoder stay
// envelope-only, because RFC 0002's own scope boundary is that "existing
// decoders remain conforming": content is opaque at the envelope layer
// (RFC 0001 §Conformance), and three normative vectors
// (unified-lock-type, unified-lock-reference,
// deprecated-angle-still-accepted) deliberately carry RETIRED
// pre-RFC-0003 spellings to prove old documents still DECODE. Making
// decode strict would break exactly the compatibility those vectors exist
// to pin. The strictness is surfaced through `hyphence validate` instead,
// whose job already is strict conformance.
//
// LOCKSTEP — rust/hyphence/src/content.rs mirrors this file rule for rule.
// The shared corpus (testdata/rfc_vectors.txt, kept byte-identical by
// checks.vectors-equality) plus the langlang cross-check in
// grammar_vectors_test.go are what keep all three — the .peg, this file,
// and the Rust twin — from drifting.
//
// PREFIX DISPATCH — the .peg's combined HyphenceContent rule exists only
// so the file has one langlang entry point; its own comment says a real
// consumer "always parses against ONE named production for a known
// PREFIX, never against HyphenceContent directly." This parser does that.
// The practical difference is HyphenceContent's trailing FreeText
// alternative, which matches any single line unconditionally: parsing
// through it would make every malformed `-`/`!`/`@` line "succeed" and
// defeat the point. grammar_vectors_test.go treats a FreeText-only match
// as a failure for the same reason.

// ErrMalformedContent is the sentinel every content-grammar syntax error
// wraps, so callers can test with errors.Is without depending on the
// concrete *ContentSyntaxError shape.
var ErrMalformedContent = errors.New("malformed metadata line content")

// ContentSyntaxError reports content that does not parse under the
// production its metadata-line prefix selects.
type ContentSyntaxError struct {
	// Prefix is the metadata line's single-character prefix.
	Prefix byte
	// Offset is a RUNE offset into the content (not a byte offset): the
	// farthest position any attempted alternative reached, which is
	// conventionally the most informative place to point at in a
	// backtracking parser.
	Offset int
	Msg    string
}

func (e *ContentSyntaxError) Error() string {
	return fmt.Sprintf(
		"malformed %q line content at offset %d: %s",
		string(e.Prefix), e.Offset, e.Msg,
	)
}

// Is reports ErrMalformedContent so errors.Is(err, ErrMalformedContent)
// matches any content syntax error.
func (e *ContentSyntaxError) Is(target error) bool {
	return target == ErrMalformedContent
}

// ParseContent reports whether content is well-formed under the content
// production RFC 0002/0003 assigns to prefix:
//
//	'!'       TypeContent    <- MarklTerm / Ident
//	'@'       BlobContent    <- String / Ident
//	'-', '<'  DashContent    <- FieldContent / RefContent
//	'#'       FreeText       <- (!LF .)*
//	'%'       OpaqueComment  <- FreeText
//
// The four structured prefixes also admit an optional TrailingComment.
// '#' and '%' are free text by design (RFC 0002 §Trailing comments do not
// apply to free text: scanning prose for a trailing '%' is undecidable and
// data-eating), so they accept anything on a single line and can only fail
// on an embedded LF.
//
// A nil return means well-formed. Errors are *ContentSyntaxError and match
// errors.Is(err, ErrMalformedContent). An unrecognized prefix returns
// ErrInvalidPrefix.
func ParseContent(prefix byte, content string) error {
	p := &contentParser{src: []rune(content)}

	switch prefix {
	case '#', '%':
		// FreeText / OpaqueComment: unconditional within one line.
		p.parseFreeText()

	case '!':
		if !p.parseTypeContent() {
			return p.syntaxError(prefix, "expected a type identifier or a markl id")
		}

		p.parseTrailingCommentOpt()

	case '@':
		if !p.parseBlobContent() {
			return p.syntaxError(prefix, "expected a blob reference: a markl id or a path")
		}

		p.parseTrailingCommentOpt()

	case '-', '<':
		if !p.parseDashContent() {
			return p.syntaxError(prefix, "expected a field predicate or a reference term")
		}

		p.parseTrailingCommentOpt()

	default:
		return ErrInvalidPrefix
	}

	if !p.atEOF() {
		return p.syntaxError(prefix, "unexpected trailing content")
	}

	return nil
}

// ---- rune classes ------------------------------------------------------

// contentReservedRunes are the literal (non-sigil) members of
// hyphence-content.peg's
//
//	Reserved <- [\[\]^=,!@<>*$~%#"'] / SigilRune
//
// Sigil runes are handled separately by isContentSigilRune wherever
// Reserved is consulted (see isContentIdentRuneAt), so this set omits them.
const contentReservedRunes = "[]^=,!@<>*$~%#\"'"

func isContentReservedRune(r rune) bool {
	return strings.ContainsRune(contentReservedRunes, r)
}

// isContentSigilRune implements SigilRune <- [:+.?].
func isContentSigilRune(r rune) bool {
	switch r {
	case ':', '+', '.', '?':
		return true
	default:
		return false
	}
}

// isContentSP1 implements SP1 <- [ \t\r\n].
func isContentSP1(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

// isContentIdentRuneAt reports whether the rune at s[i] is consumed as
// identifier content, per
//
//	IdentRune <- '-' / '/' / (SigilRune &IdentRune) / (!Reserved !SP1 .)
//
// This is THE STRICT SIGIL RULE: a sigil rune is identifier-interior only
// when the rune immediately following it is itself identifier content —
// the grammar's `&IdentRune` positive lookahead, implemented here by
// recursing on i+1. A trailing run of sigil runes has nothing
// identifier-shaped left to look ahead to, so it bottoms out false, which
// is exactly what makes it a term-final sigil suffix rather than
// identifier content: `todo:` is identifier "todo" plus sigil ":", while
// `caldav:fastmail` is one identifier because ':' is followed by more
// identifier content.
//
// '-' and '/' are unconditional identifier runes with no lookahead, so a
// trailing hyphen or slash is never stripped as if it were a sigil.
func isContentIdentRuneAt(s []rune, i int) bool {
	if i < 0 || i >= len(s) {
		return false
	}

	switch r := s[i]; {
	case r == '-' || r == '/':
		return true
	case isContentSigilRune(r):
		return isContentIdentRuneAt(s, i+1)
	case isContentReservedRune(r) || isContentSP1(r):
		return false
	default:
		return true
	}
}

// isContentFormatChar implements marklid.peg's FormatChar <- [a-zA-Z0-9_],
// imported by hyphence-content.peg as part of Format. Note it excludes
// '-', which is what makes Digest's `Format '-' DataChar+` split
// unambiguous even for a format id carrying underscores
// (`ssh_ecdsa_nistp256_pub-qqxyz`).
func isContentFormatChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_'
}

// isContentDataChar implements marklid.peg's
//
//	DataChar <- [qpzry9x8gf2tvdw0s3jn54khce6mua7l]
//
// the 32-character blech32 alphabet a markl-id data payload may contain
// (piggy RFC 0011 §3.1). It notably OMITS 'b', 'i', 'o', and '1' — the
// characters bech32 drops as visually ambiguous — which is the whole
// reason the strict digest slot rejects junk like `blake2b256-9bt3`.
func isContentDataChar(r rune) bool {
	switch r {
	case 'q', 'p', 'z', 'r', 'y', '9', 'x', '8', 'g', 'f', '2', 't', 'v',
		'd', 'w', '0', 's', '3', 'j', 'n', '5', '4', 'k', 'h', 'c', 'e',
		'6', 'm', 'u', 'a', '7', 'l':
		return true
	default:
		return false
	}
}

// ---- parser ------------------------------------------------------------

// contentParser is a hand-rolled recursive-descent, backtracking parser
// over hyphence-content.peg. Each grammar rule has a corresponding parseX
// method; ordered choices try alternatives in the grammar's own order and
// restore pos on failure, mirroring PEG's
// ordered-choice-with-backtracking semantics. Methods return a bool rather
// than an error: only ParseContent builds a diagnostic, from the farthest
// position any sub-parse reached.
type contentParser struct {
	src []rune
	pos int

	farthest    int
	farthestMsg string
}

func (p *contentParser) atEOF() bool { return p.pos >= len(p.src) }

// fail records a candidate diagnostic. The farthest position wins, since
// in a backtracking parser the alternative that got furthest is almost
// always the one the author meant.
func (p *contentParser) fail(msg string) {
	if p.pos >= p.farthest {
		p.farthest = p.pos
		p.farthestMsg = msg
	}
}

func (p *contentParser) syntaxError(prefix byte, fallback string) error {
	msg, offset := p.farthestMsg, p.farthest
	if msg == "" {
		msg, offset = fallback, p.pos
	}

	return &ContentSyntaxError{Prefix: prefix, Offset: offset, Msg: msg}
}

// at reports whether the rune at the cursor is r.
func (p *contentParser) at(r rune) bool {
	return !p.atEOF() && p.src[p.pos] == r
}

// skipSPOpt consumes SP? — zero or more whitespace runes; always succeeds.
//
// SP is optional everywhere it appears in this grammar and meaning-bearing
// nowhere, but it still has to be CONSUMED where present: a genuinely
// spaced input would otherwise be left unconsumed at the closing EOF.
func (p *contentParser) skipSPOpt() {
	for !p.atEOF() && isContentSP1(p.src[p.pos]) {
		p.pos++
	}
}

// ---- lexical rules -----------------------------------------------------

// parseIdentText implements Ident <- IdentRune+, and equally
// Bareword <- IdentRune+: the two rules have identical bodies under
// different names, so one scanner serves both.
func (p *contentParser) parseIdentText() bool {
	start := p.pos
	for !p.atEOF() && isContentIdentRuneAt(p.src, p.pos) {
		p.pos++
	}

	if p.pos == start {
		p.fail("expected an identifier")
		return false
	}

	return true
}

// parseString implements the imported piggy rules
//
//	String <- '"' (!'"' Char)* '"' / "'" (!"'" Char)* "'"
//	Char   <- '\\' . / .
//
// A backslash consumes the following rune, so an escaped quote does not
// close the string. A trailing lone backslash is consumed by Char's second
// alternative and then fails on the missing close quote, matching the PEG.
func (p *contentParser) parseString() bool {
	save := p.pos
	if p.atEOF() {
		p.fail("expected a quoted string")
		return false
	}

	quote := p.src[p.pos]
	if quote != '"' && quote != '\'' {
		p.fail("expected a quoted string")
		return false
	}

	p.pos++

	for {
		if p.atEOF() {
			p.pos = save
			p.fail("unterminated quoted string")

			return false
		}

		if p.src[p.pos] == quote {
			p.pos++
			return true
		}

		if p.src[p.pos] == '\\' && p.pos+1 < len(p.src) {
			p.pos += 2
			continue
		}

		p.pos++
	}
}

// parseDigest implements
//
//	Digest <- Format '-' DataChar+ !IdentRune
//
// composed from piggy's imported Format and DataChar primitives.
// Charset-strict but LENGTH-AGNOSTIC: a full digest and an abbreviated
// prefix differ only in DataChar count, and length/checksum completeness
// stays a decoder concern per piggy RFC 0011 §4.1 (a PEG cannot compute a
// BCH checksum anyway).
//
// The trailing !IdentRune anchor is what rejects junk a bare DataChar+
// would leave as unconsumed garbage: `blake2b256-9bt3` fails because 'b'
// is outside blech32 and therefore an IdentRune sitting immediately after
// the '9' DataChar run, so this refuses rather than silently accepting a
// truncated '9'.
func (p *contentParser) parseDigest() bool {
	save := p.pos

	formatStart := p.pos
	for !p.atEOF() && isContentFormatChar(p.src[p.pos]) {
		p.pos++
	}

	if p.pos == formatStart {
		p.pos = save
		p.fail("expected a markl-id format")

		return false
	}

	if !p.at('-') {
		p.pos = save
		p.fail("expected '-' between a markl id's format and data")

		return false
	}

	p.pos++

	dataStart := p.pos
	for !p.atEOF() && isContentDataChar(p.src[p.pos]) {
		p.pos++
	}

	if p.pos == dataStart {
		p.pos = save
		p.fail("expected markl-id digest data in the blech32 alphabet")

		return false
	}

	if !p.atEOF() && isContentIdentRuneAt(p.src, p.pos) {
		p.pos = save
		p.fail("markl-id digest data contains a character outside the blech32 alphabet")

		return false
	}

	return true
}

// parseDigestTerm implements DigestTerm <- '@' Digest — blob identity,
// content-addressed exact match, with a term-initial '@'.
func (p *contentParser) parseDigestTerm() bool {
	save := p.pos
	if !p.at('@') {
		p.fail("expected '@' introducing a digest term")
		return false
	}

	p.pos++

	if !p.parseDigest() {
		p.pos = save
		return false
	}

	return true
}

// parseMarklTerm implements MarklTerm <- (String / Ident) '@' Digest — one
// atomic two-slot purpose-full markl id.
//
// Because the purpose slot tries String first, a quoted purpose that
// itself contains '@' works out naturally: in `"a@b"@blake2b256-xyz` the
// String consumes `"a@b"` and the join is the SECOND '@'. That is the
// quote-aware scan piggy RFC 0011 §2.2 requires — locating the join with
// the FIRST '@' would split this id wrongly, and it is the single most
// divergence-prone point across the implementations.
func (p *contentParser) parseMarklTerm() bool {
	save := p.pos

	if !p.parseString() {
		p.pos = save
		if !p.parseIdentText() {
			p.pos = save
			return false
		}
	}

	if !p.at('@') {
		p.pos = save
		p.fail("expected '@' joining a markl id's purpose and digest")

		return false
	}

	p.pos++

	if !p.parseDigest() {
		p.pos = save
		return false
	}

	return true
}

// ---- per-line-kind rules -----------------------------------------------

// parseTypeContent implements TypeContent <- MarklTerm / Ident, the '!'
// line's content.
func (p *contentParser) parseTypeContent() bool {
	save := p.pos
	if p.parseMarklTerm() {
		return true
	}

	p.pos = save

	return p.parseIdentText()
}

// parseBlobContent implements BlobContent <- String / Ident, the '@' line's
// content: a markl id, or a file path (quoted if it contains spaces or
// reserved runes).
//
// Note this rule does NOT go through Digest. The '@' PREFIX is the line's
// operator, not a DigestTerm's term-initial '@', and the value may be a
// path rather than a digest, so it stays a plain identifier or string.
func (p *contentParser) parseBlobContent() bool {
	save := p.pos
	if p.parseString() {
		return true
	}

	p.pos = save

	return p.parseIdentText()
}

// parseDashContent implements DashContent <- FieldContent / RefContent,
// the content of a '-' line and of its deprecated '<' synonym (RFC 0002
// §`<` Deprecation gives them identical content grammar).
//
// FieldContent is tried first because its branch is the only shape that
// can require an '=': RefContent's bare-term branches would otherwise
// short-circuit on the field name and leave `="..."` unconsumed.
func (p *contentParser) parseDashContent() bool {
	save := p.pos
	if p.parseFieldContent() {
		return true
	}

	p.pos = save

	return p.parseRefContent()
}

// parseRefContent implements RefContent <- GroundTerm and
// GroundTerm <- MarklTerm / String / Ident.
func (p *contentParser) parseRefContent() bool {
	save := p.pos
	if p.parseMarklTerm() {
		return true
	}

	p.pos = save
	if p.parseString() {
		return true
	}

	p.pos = save

	return p.parseIdentText()
}

// parseFieldContent implements
//
//	FieldContent <- FieldName SP? '=' SP? FieldRHS
//
// The optional SP around '=' is RFC 0003 §Canonical emission's spacing.
func (p *contentParser) parseFieldContent() bool {
	save := p.pos

	if !p.parseFieldName() {
		p.pos = save
		return false
	}

	p.skipSPOpt()

	if !p.at('=') {
		p.pos = save
		p.fail("expected '=' in a field predicate")

		return false
	}

	p.pos++
	p.skipSPOpt()

	if !p.parseFieldRHS() {
		p.pos = save
		return false
	}

	return true
}

// parseFieldName implements FieldName <- String / Ident — a flat per-type
// field namespace, where a quoted field name is opaque.
//
// Its body is identical to parseBlobContent's today. That duplication is
// deliberate, not an oversight to collapse: this file is organized so every
// .peg rule has a same-named method, which is what lets a reader diff it
// against the grammar rule by rule. These are two distinct rules governing
// unrelated things (a field namespace vs. a blob path) that happen to
// coincide, and either can diverge without the other. Ident/Bareword share
// one scanner only because the grammar gives them the same body under two
// names for readability, which is a different situation.
func (p *contentParser) parseFieldName() bool {
	save := p.pos
	if p.parseString() {
		return true
	}

	p.pos = save

	return p.parseIdentText()
}

// parseFieldRHS implements
//
//	FieldRHS   <- DigestTerm / FieldValue
//	FieldValue <- MarklTerm / String / Bareword
//
// DigestTerm comes first so an id-less field lock (`_base=@blake2b256-…`)
// is read as a digest term rather than failing into the value branches.
func (p *contentParser) parseFieldRHS() bool {
	save := p.pos
	if p.parseDigestTerm() {
		return true
	}

	p.pos = save
	if p.parseMarklTerm() {
		return true
	}

	p.pos = save
	if p.parseString() {
		return true
	}

	p.pos = save

	// Bareword <- IdentRune+ — the same body as Ident.
	return p.parseIdentText()
}

// parseTrailingCommentOpt implements TrailingComment? where
//
//	TrailingComment <- SP? '%' (!LF .)*
//
// The SP is OPTIONAL, not required (ruling 2026-07-20): '%' is Reserved
// and so self-delimiting against every content production, which makes a
// glued `md%comment` and a spaced `md % comment` parse identically. The
// SP? still has to exist to consume the space in the spaced form — in a
// PEG, dropping it would FORBID the space rather than permit glue.
func (p *contentParser) parseTrailingCommentOpt() {
	save := p.pos

	p.skipSPOpt()

	if !p.at('%') {
		p.pos = save
		return
	}

	p.pos++
	p.parseFreeText()
}

// parseFreeText implements FreeText <- (!LF .)*, which is also
// OpaqueComment's body and a trailing comment's tail. LF is RFC 0001's own
// line terminator, so this stops at one rather than consuming it.
func (p *contentParser) parseFreeText() {
	for !p.atEOF() && p.src[p.pos] != '\n' {
		p.pos++
	}
}
