package hyphence

import (
	"errors"
	"fmt"
	"strings"
)

// This file is the Go implementation of docs/rfcs/hyphence-content.peg —
// the content grammar RFC 0002 defines and RFC 0003 amends. It decides
// whether a metadata line's content is well-formed under the production
// its PREFIX selects, reports where and why when it is not, and — for the
// typed head model in head.go — returns what it parsed. There is ONE
// parser: ParseContent is the typed parse (parseLine) with its result
// discarded, so the recognizer and the typed reader cannot disagree about
// a line.
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
// whose job already is strict conformance, and through HeadFromDocument /
// DecodeHead, which opt in by reading typed content.
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
	_, err := parseLine(prefix, content)
	return err
}

// parsedLine is one metadata line's content, parsed: the typed line plus
// its trailing comment, if any. isComment marks a '%' comment line, which
// carries no typed content of its own (line and trailingComment are then
// zero).
type parsedLine struct {
	line            Line
	trailingComment *string
	isComment       bool
}

// parseLine is the typed parse behind ParseContent and HeadFromDocument.
// Errors are as ParseContent documents them.
func parseLine(prefix byte, content string) (parsedLine, error) {
	p := &contentParser{src: []rune(content)}

	var parsed parsedLine

	switch prefix {
	case '#':
		// FreeText: unconditional within one line.
		parsed.line = Line{Kind: LineDescription, Text: p.parseFreeText()}

	case '%':
		// OpaqueComment: unconditional within one line.
		p.parseFreeText()
		parsed.isComment = true

	case '!':
		line, ok := p.parseTypeContent()
		if !ok {
			return parsedLine{}, p.syntaxError(prefix, "expected a type identifier or a markl id")
		}

		parsed.line = line
		parsed.trailingComment = p.trailingComment()

	case '@':
		blob, ok := p.parseBlobContent()
		if !ok {
			return parsedLine{}, p.syntaxError(prefix, "expected a blob reference: a markl id or a path")
		}

		parsed.line = Line{Kind: LineBlob, Text: blob}
		parsed.trailingComment = p.trailingComment()

	case '-', '<':
		line, ok := p.parseDashContent()
		if !ok {
			return parsedLine{}, p.syntaxError(prefix, "expected a field predicate or a reference term")
		}

		parsed.line = line
		parsed.trailingComment = p.trailingComment()

	default:
		return parsedLine{}, ErrInvalidPrefix
	}

	if !p.atEOF() {
		return parsedLine{}, p.syntaxError(prefix, "unexpected trailing content")
	}

	return parsed, nil
}

// trailingComment parses the optional TrailingComment after a structured
// line's content into parsedLine's pointer form.
func (p *contentParser) trailingComment() *string {
	comment, ok := p.parseTrailingCommentOpt()
	if !ok {
		return nil
	}

	return &comment
}

// isBareIdent reports whether s is expressible unquoted as an Ident
// (equally a Bareword or a bare FieldName): non-empty, and every rune
// consumed as identifier content under the strict sigil rule. The
// canonical encoder writes a text bare exactly when this holds.
func isBareIdent(s string) bool {
	runes := []rune(s)
	if len(runes) == 0 {
		return false
	}

	for i := range runes {
		if !isContentIdentRuneAt(runes, i) {
			return false
		}
	}

	return true
}

// isDigestParts reports whether format and data form a well-formed Digest
// slot: a non-empty Format run, and a non-empty DataChar run
// (charset-strict, length-agnostic).
func isDigestParts(format, data string) bool {
	if format == "" || data == "" {
		return false
	}

	for _, r := range format {
		if !isContentFormatChar(r) {
			return false
		}
	}

	for _, r := range data {
		if !isContentDataChar(r) {
			return false
		}
	}

	return true
}

// unescapeQuotedInterior applies the Doddish unescape rules piggy-markl
// applies to a quoted slot: \n \t \r \a \b \f \v name their control
// characters, and any other escaped rune stands for itself (which is what
// makes \" and \\ work).
func unescapeQuotedInterior(interior []rune) string {
	var out strings.Builder
	out.Grow(len(interior))

	for i := 0; i < len(interior); i++ {
		r := interior[i]
		if r != '\\' {
			out.WriteRune(r)
			continue
		}

		i++
		if i >= len(interior) {
			out.WriteRune('\\')
			break
		}

		switch escaped := interior[i]; escaped {
		case 'n':
			out.WriteRune('\n')
		case 't':
			out.WriteRune('\t')
		case 'r':
			out.WriteRune('\r')
		case 'a':
			out.WriteRune('\a')
		case 'b':
			out.WriteRune('\b')
		case 'f':
			out.WriteRune('\f')
		case 'v':
			out.WriteRune('\v')
		default:
			out.WriteRune(escaped)
		}
	}

	return out.String()
}

// ---- rune classes ------------------------------------------------------

// contentReservedRunes are the literal (non-sigil) members of
// hyphence-content.peg's
//
//	Reserved <- [\[\]^=,!@<>*$~%#"'()] / SigilRune
//
// Sigil runes are handled separately by isContentSigilRune wherever
// Reserved is consulted (see isContentIdentRuneAt), so this set omits them.
// '(' and ')' were added 2026-08-30 so a parenthetical is self-delimiting,
// the precondition for trellis (cutting-garden RFC 0014) to define `(…)`
// as a META QUALIFIER term (cutting-garden native-tags G10).
const contentReservedRunes = "[]^=,!@<>*$~%#\"'()"

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
// ordered-choice-with-backtracking semantics. Methods return what they
// parsed plus an ok bool rather than an error: only parseLine builds a
// diagnostic, from the farthest position any sub-parse reached.
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
func (p *contentParser) parseIdentText() (string, bool) {
	start := p.pos
	for !p.atEOF() && isContentIdentRuneAt(p.src, p.pos) {
		p.pos++
	}

	if p.pos == start {
		p.fail("expected an identifier")
		return "", false
	}

	return string(p.src[start:p.pos]), true
}

// parseString implements the imported piggy rules
//
//	String <- '"' (!'"' Char)* '"' / "'" (!"'" Char)* "'"
//	Char   <- '\\' . / .
//
// A backslash consumes the following rune, so an escaped quote does not
// close the string. A trailing lone backslash is consumed by Char's second
// alternative and then fails on the missing close quote, matching the PEG.
//
// Returns the UNESCAPED interior (see unescapeQuotedInterior).
func (p *contentParser) parseString() (string, bool) {
	save := p.pos
	if p.atEOF() {
		p.fail("expected a quoted string")
		return "", false
	}

	quote := p.src[p.pos]
	if quote != '"' && quote != '\'' {
		p.fail("expected a quoted string")
		return "", false
	}

	p.pos++
	interiorStart := p.pos

	for {
		if p.atEOF() {
			p.pos = save
			p.fail("unterminated quoted string")

			return "", false
		}

		if p.src[p.pos] == quote {
			interior := unescapeQuotedInterior(p.src[interiorStart:p.pos])
			p.pos++

			return interior, true
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
func (p *contentParser) parseDigest() (MarklDigest, bool) {
	save := p.pos

	formatStart := p.pos
	for !p.atEOF() && isContentFormatChar(p.src[p.pos]) {
		p.pos++
	}

	if p.pos == formatStart {
		p.pos = save
		p.fail("expected a markl-id format")

		return MarklDigest{}, false
	}

	formatEnd := p.pos

	if !p.at('-') {
		p.pos = save
		p.fail("expected '-' between a markl id's format and data")

		return MarklDigest{}, false
	}

	p.pos++

	dataStart := p.pos
	for !p.atEOF() && isContentDataChar(p.src[p.pos]) {
		p.pos++
	}

	if p.pos == dataStart {
		p.pos = save
		p.fail("expected markl-id digest data in the blech32 alphabet")

		return MarklDigest{}, false
	}

	if !p.atEOF() && isContentIdentRuneAt(p.src, p.pos) {
		p.pos = save
		p.fail("markl-id digest data contains a character outside the blech32 alphabet")

		return MarklDigest{}, false
	}

	return MarklDigest{
		format: string(p.src[formatStart:formatEnd]),
		data:   string(p.src[dataStart:p.pos]),
	}, true
}

// parseDigestTerm implements DigestTerm <- '@' Digest — blob identity,
// content-addressed exact match, with a term-initial '@'.
func (p *contentParser) parseDigestTerm() (MarklDigest, bool) {
	save := p.pos
	if !p.at('@') {
		p.fail("expected '@' introducing a digest term")
		return MarklDigest{}, false
	}

	p.pos++

	digest, ok := p.parseDigest()
	if !ok {
		p.pos = save
		return MarklDigest{}, false
	}

	return digest, true
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
func (p *contentParser) parseMarklTerm() (MarklID, bool) {
	save := p.pos

	purpose, ok := p.parseString()
	if !ok {
		p.pos = save

		if purpose, ok = p.parseIdentText(); !ok {
			p.pos = save
			return MarklID{}, false
		}
	}

	if !p.at('@') {
		p.pos = save
		p.fail("expected '@' joining a markl id's purpose and digest")

		return MarklID{}, false
	}

	p.pos++

	digest, ok := p.parseDigest()
	if !ok {
		p.pos = save
		return MarklID{}, false
	}

	return MarklID{Purpose: &purpose, Digest: digest}, true
}

// ---- per-line-kind rules -----------------------------------------------

// parseTypeContent implements TypeContent <- MarklTerm / Ident, the '!'
// line's content.
func (p *contentParser) parseTypeContent() (Line, bool) {
	save := p.pos
	if id, ok := p.parseMarklTerm(); ok {
		lock := id.Digest
		return Line{Kind: LineType, Name: *id.Purpose, Lock: &lock}, true
	}

	p.pos = save

	name, ok := p.parseIdentText()
	if !ok {
		return Line{}, false
	}

	return Line{Kind: LineType, Name: name}, true
}

// parseBlobContent implements BlobContent <- String / Ident, the '@' line's
// content: a markl id, or a file path (quoted if it contains spaces or
// reserved runes).
//
// Note this rule does NOT go through Digest. The '@' PREFIX is the line's
// operator, not a DigestTerm's term-initial '@', and the value may be a
// path rather than a digest, so it stays a plain identifier or string.
func (p *contentParser) parseBlobContent() (string, bool) {
	save := p.pos
	if path, ok := p.parseString(); ok {
		return path, true
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
func (p *contentParser) parseDashContent() (Line, bool) {
	save := p.pos
	if name, value, ok := p.parseFieldContent(); ok {
		return Line{Kind: LineField, Name: name, Value: value}, true
	}

	p.pos = save

	term, ok := p.parseRefContent()
	if !ok {
		return Line{}, false
	}

	return Line{Kind: LineTerm, Value: term}, true
}

// parseRefContent implements RefContent <- GroundTerm and
// GroundTerm <- MarklTerm / String / Ident.
func (p *contentParser) parseRefContent() (Value, bool) {
	save := p.pos
	if id, ok := p.parseMarklTerm(); ok {
		return Value{Markl: &id}, true
	}

	p.pos = save
	if text, ok := p.parseString(); ok {
		return Value{Text: text}, true
	}

	p.pos = save

	text, ok := p.parseIdentText()
	if !ok {
		return Value{}, false
	}

	return Value{Text: text}, true
}

// parseFieldContent implements
//
//	FieldContent <- FieldName SP? '=' SP? FieldRHS
//
// The optional SP around '=' is RFC 0003 §Canonical emission's spacing.
func (p *contentParser) parseFieldContent() (string, Value, bool) {
	save := p.pos

	name, ok := p.parseFieldName()
	if !ok {
		p.pos = save
		return "", Value{}, false
	}

	p.skipSPOpt()

	if !p.at('=') {
		p.pos = save
		p.fail("expected '=' in a field predicate")

		return "", Value{}, false
	}

	p.pos++
	p.skipSPOpt()

	value, ok := p.parseFieldRHS()
	if !ok {
		p.pos = save
		return "", Value{}, false
	}

	return name, value, true
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
func (p *contentParser) parseFieldName() (string, bool) {
	save := p.pos
	if name, ok := p.parseString(); ok {
		return name, true
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
func (p *contentParser) parseFieldRHS() (Value, bool) {
	save := p.pos
	if digest, ok := p.parseDigestTerm(); ok {
		return Value{Markl: &MarklID{Digest: digest}}, true
	}

	p.pos = save
	if id, ok := p.parseMarklTerm(); ok {
		return Value{Markl: &id}, true
	}

	p.pos = save
	if text, ok := p.parseString(); ok {
		return Value{Text: text}, true
	}

	p.pos = save

	// Bareword <- IdentRune+ — the same body as Ident.
	text, ok := p.parseIdentText()
	if !ok {
		return Value{}, false
	}

	return Value{Text: text}, true
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
//
// Returns the comment's text — everything after the '%', verbatim — and
// whether a trailing comment was present.
func (p *contentParser) parseTrailingCommentOpt() (string, bool) {
	save := p.pos

	p.skipSPOpt()

	if !p.at('%') {
		p.pos = save
		return "", false
	}

	p.pos++

	return p.parseFreeText(), true
}

// parseFreeText implements FreeText <- (!LF .)*, which is also
// OpaqueComment's body and a trailing comment's tail. LF is RFC 0001's own
// line terminator, so this stops at one rather than consuming it.
func (p *contentParser) parseFreeText() string {
	start := p.pos
	for !p.atEOF() && p.src[p.pos] != '\n' {
		p.pos++
	}

	return string(p.src[start:p.pos])
}
