//! The RFC 0002/0003 content grammar — the Rust mirror of
//! `docs/rfcs/hyphence-content.peg` and of `go/hyphence/content.go`.
//!
//! This is a RECOGNIZER: it decides whether a metadata line's content is
//! well-formed under the production its [`Prefix`] selects, and reports where and
//! why when it is not. It deliberately builds no syntax tree — no caller needs
//! one yet, and an unused AST would be two more surfaces (here and in Go) to keep
//! in lockstep for nothing. Adding one later is additive.
//!
//! # Scope
//!
//! This is NOT wired into [`crate::Document::decode`]. The envelope decoder stays
//! content-opaque, because RFC 0002's own scope boundary is that "existing
//! decoders remain conforming" (RFC 0001 §Conformance), and three normative
//! vectors — `unified-lock-type`, `unified-lock-reference`, and
//! `deprecated-angle-still-accepted` — deliberately carry RETIRED pre-RFC-0003
//! spellings to prove old documents still DECODE. Making decode strict would
//! break exactly the compatibility those vectors exist to pin. Callers that want
//! content strictness opt in by calling [`parse_content`]; the Go side surfaces
//! the same choice as `MetadataValidator::CheckContent`, which `hyphence
//! validate` sets.
//!
//! # Lockstep
//!
//! `go/hyphence/content.go` mirrors this file rule for rule. The shared corpus
//! (`testdata/rfc_vectors.txt`, kept byte-identical across the two impls by
//! `checks.vectors-equality`) plus the langlang cross-check on the Go side are
//! what keep all three — the `.peg`, the Go parser, and this one — from drifting.
//! `conformance.rs` runs the corpus through this parser for that reason.
//!
//! # Prefix dispatch
//!
//! The `.peg`'s combined `HyphenceContent` rule exists only so the file has one
//! langlang entry point; its own comment says a real consumer "always parses
//! against ONE named production for a known PREFIX, never against
//! HyphenceContent directly." This parser does that. The practical difference is
//! `HyphenceContent`'s trailing `FreeText` alternative, which matches any single
//! line unconditionally: going through it would make every malformed `-`/`!`/`@`
//! line "succeed" and defeat the point.

use crate::document::Prefix;
use std::fmt;

/// Content that does not parse under the production its metadata-line prefix
/// selects.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct ContentError {
    /// The metadata line's prefix, which selected the production.
    pub prefix: Prefix,
    /// A `char` offset into the content (not a byte offset): the farthest
    /// position any attempted alternative reached, which is conventionally the
    /// most informative place to point at in a backtracking parser.
    pub offset: usize,
    /// What was expected at `offset`.
    pub message: &'static str,
}

impl fmt::Display for ContentError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "malformed '{}' line content at offset {}: {}",
            self.prefix.byte() as char,
            self.offset,
            self.message
        )
    }
}

impl std::error::Error for ContentError {}

/// Check `content` against the content production RFC 0002/0003 assigns to
/// `prefix`:
///
/// | prefix     | production                                |
/// |------------|-------------------------------------------|
/// | `!`        | `TypeContent  <- MarklTerm / Ident`       |
/// | `@`        | `BlobContent  <- String / Ident`          |
/// | `-` / `<`  | `DashContent  <- FieldContent / RefContent` |
/// | `#`        | `FreeText     <- (!LF .)*`                |
/// | `%`        | `OpaqueComment <- FreeText`               |
///
/// The four structured prefixes also admit an optional `TrailingComment`. `#`
/// and `%` are free text by design (RFC 0002 §Trailing comments do not apply to
/// free text: scanning prose for a trailing `%` is undecidable and data-eating),
/// so they accept anything on a single line and can only fail on an embedded LF.
///
/// # Errors
///
/// Returns [`ContentError`] with the offset and reason of the farthest failure.
pub fn parse_content(prefix: Prefix, content: &str) -> Result<(), ContentError> {
    let mut p = ContentParser::new(content);

    match prefix {
        // FreeText / OpaqueComment: unconditional within one line.
        Prefix::Description | Prefix::Comment => {
            p.free_text();
        }
        Prefix::Type => {
            if !p.type_content() {
                return Err(p.error(prefix, "expected a type identifier or a markl id"));
            }
            p.trailing_comment_opt();
        }
        Prefix::Blob => {
            if !p.blob_content() {
                return Err(p.error(prefix, "expected a blob reference: a markl id or a path"));
            }
            p.trailing_comment_opt();
        }
        Prefix::TagRef | Prefix::ObjectRef => {
            if !p.dash_content() {
                return Err(p.error(prefix, "expected a field predicate or a reference term"));
            }
            p.trailing_comment_opt();
        }
    }

    if !p.at_eof() {
        return Err(p.error(prefix, "unexpected trailing content"));
    }

    Ok(())
}

// ---- rune classes ---------------------------------------------------------

/// The literal (non-sigil) members of the grammar's
/// `Reserved <- [\[\]^=,!@<>*$~%#"'] / SigilRune`.
///
/// Sigil runes are handled separately by [`is_sigil_rune`] wherever `Reserved` is
/// consulted (see [`is_ident_rune_at`]), so this set omits them.
const RESERVED_RUNES: &str = "[]^=,!@<>*$~%#\"'";

fn is_reserved_rune(c: char) -> bool {
    RESERVED_RUNES.contains(c)
}

/// `SigilRune <- [:+.?]`
fn is_sigil_rune(c: char) -> bool {
    matches!(c, ':' | '+' | '.' | '?')
}

/// `SP1 <- [ \t\r\n]`
fn is_sp1(c: char) -> bool {
    matches!(c, ' ' | '\t' | '\r' | '\n')
}

/// Whether the char at `s[i]` is consumed as identifier content, per
///
/// ```text
/// IdentRune <- '-' / '/' / (SigilRune &IdentRune) / (!Reserved !SP1 .)
/// ```
///
/// This is THE STRICT SIGIL RULE: a sigil rune is identifier-interior only when
/// the char immediately following it is itself identifier content — the grammar's
/// `&IdentRune` positive lookahead, implemented here by recursing on `i + 1`. A
/// trailing run of sigil runes has nothing identifier-shaped left to look ahead
/// to, so it bottoms out false, which is exactly what makes it a term-final sigil
/// suffix rather than identifier content: `todo:` is identifier `todo` plus sigil
/// `:`, while `caldav:fastmail` is one identifier because `:` is followed by more
/// identifier content.
///
/// `-` and `/` are unconditional identifier runes with no lookahead, so a
/// trailing hyphen or slash is never stripped as if it were a sigil.
fn is_ident_rune_at(s: &[char], i: usize) -> bool {
    let Some(&c) = s.get(i) else {
        return false;
    };

    if c == '-' || c == '/' {
        true
    } else if is_sigil_rune(c) {
        is_ident_rune_at(s, i + 1)
    } else {
        !(is_reserved_rune(c) || is_sp1(c))
    }
}

/// marklid.peg's `FormatChar <- [a-zA-Z0-9_]`, imported by hyphence-content.peg
/// as part of `Format`. Note it excludes `-`, which is what makes `Digest`'s
/// `Format '-' DataChar+` split unambiguous even for a format id carrying
/// underscores (`ssh_ecdsa_nistp256_pub-qqxyz`).
fn is_format_char(c: char) -> bool {
    c.is_ascii_alphanumeric() || c == '_'
}

/// marklid.peg's `DataChar <- [qpzry9x8gf2tvdw0s3jn54khce6mua7l]` — the
/// 32-character blech32 alphabet a markl-id data payload may contain (piggy RFC
/// 0011 §3.1).
///
/// It notably OMITS `b`, `i`, `o`, and `1` — the characters bech32 drops as
/// visually ambiguous — which is the whole reason the strict digest slot rejects
/// junk like `blake2b256-9bt3`.
fn is_data_char(c: char) -> bool {
    matches!(
        c,
        'q' | 'p'
            | 'z'
            | 'r'
            | 'y'
            | '9'
            | 'x'
            | '8'
            | 'g'
            | 'f'
            | '2'
            | 't'
            | 'v'
            | 'd'
            | 'w'
            | '0'
            | 's'
            | '3'
            | 'j'
            | 'n'
            | '5'
            | '4'
            | 'k'
            | 'h'
            | 'c'
            | 'e'
            | '6'
            | 'm'
            | 'u'
            | 'a'
            | '7'
            | 'l'
    )
}

// ---- parser ---------------------------------------------------------------

/// A hand-rolled recursive-descent, backtracking parser over
/// hyphence-content.peg. Each grammar rule has a corresponding method; ordered
/// choices try alternatives in the grammar's own order and restore `pos` on
/// failure, mirroring PEG's ordered-choice-with-backtracking semantics. Methods
/// return `bool` rather than a `Result`: only [`parse_content`] builds a
/// diagnostic, from the farthest position any sub-parse reached.
struct ContentParser {
    src: Vec<char>,
    pos: usize,

    farthest: usize,
    farthest_msg: Option<&'static str>,
}

impl ContentParser {
    fn new(content: &str) -> ContentParser {
        ContentParser {
            src: content.chars().collect(),
            pos: 0,
            farthest: 0,
            farthest_msg: None,
        }
    }

    fn at_eof(&self) -> bool {
        self.pos >= self.src.len()
    }

    /// Whether the char at the cursor is `c`.
    fn at(&self, c: char) -> bool {
        self.src.get(self.pos) == Some(&c)
    }

    /// Record a candidate diagnostic. The farthest position wins, since in a
    /// backtracking parser the alternative that got furthest is almost always
    /// the one the author meant.
    fn fail(&mut self, msg: &'static str) {
        if self.pos >= self.farthest {
            self.farthest = self.pos;
            self.farthest_msg = Some(msg);
        }
    }

    fn error(&self, prefix: Prefix, fallback: &'static str) -> ContentError {
        match self.farthest_msg {
            Some(msg) => ContentError {
                prefix,
                offset: self.farthest,
                message: msg,
            },
            None => ContentError {
                prefix,
                offset: self.pos,
                message: fallback,
            },
        }
    }

    /// Consume `SP?` — zero or more whitespace chars; always succeeds.
    ///
    /// SP is optional everywhere it appears in this grammar and meaning-bearing
    /// nowhere, but it still has to be CONSUMED where present: a genuinely
    /// spaced input would otherwise be left unconsumed at the closing EOF.
    fn skip_sp_opt(&mut self) {
        while self.src.get(self.pos).is_some_and(|&c| is_sp1(c)) {
            self.pos += 1;
        }
    }

    // ---- lexical rules ----------------------------------------------------

    /// `Ident <- IdentRune+`, and equally `Bareword <- IdentRune+`: the two rules
    /// have identical bodies under different names, so one scanner serves both.
    fn ident_text(&mut self) -> bool {
        let start = self.pos;
        while is_ident_rune_at(&self.src, self.pos) {
            self.pos += 1;
        }

        if self.pos == start {
            self.fail("expected an identifier");
            return false;
        }

        true
    }

    /// The imported piggy rules
    ///
    /// ```text
    /// String <- '"' (!'"' Char)* '"' / "'" (!"'" Char)* "'"
    /// Char   <- '\\' . / .
    /// ```
    ///
    /// A backslash consumes the following char, so an escaped quote does not
    /// close the string. A trailing lone backslash is consumed by `Char`'s second
    /// alternative and then fails on the missing close quote, matching the PEG.
    fn string(&mut self) -> bool {
        let save = self.pos;

        let Some(&quote) = self.src.get(self.pos) else {
            self.fail("expected a quoted string");
            return false;
        };

        if quote != '"' && quote != '\'' {
            self.fail("expected a quoted string");
            return false;
        }

        self.pos += 1;

        loop {
            let Some(&c) = self.src.get(self.pos) else {
                self.pos = save;
                self.fail("unterminated quoted string");

                return false;
            };

            if c == quote {
                self.pos += 1;
                return true;
            }

            if c == '\\' && self.pos + 1 < self.src.len() {
                self.pos += 2;
                continue;
            }

            self.pos += 1;
        }
    }

    /// ```text
    /// Digest <- Format '-' DataChar+ !IdentRune
    /// ```
    ///
    /// composed from piggy's imported `Format` and `DataChar` primitives.
    /// Charset-strict but LENGTH-AGNOSTIC: a full digest and an abbreviated
    /// prefix differ only in `DataChar` count, and length/checksum completeness
    /// stays a decoder concern per piggy RFC 0011 §4.1 (a PEG cannot compute a
    /// BCH checksum anyway).
    ///
    /// The trailing `!IdentRune` anchor is what rejects junk a bare `DataChar+`
    /// would leave as unconsumed garbage: `blake2b256-9bt3` fails because `b` is
    /// outside blech32 and therefore an `IdentRune` sitting immediately after the
    /// `9` `DataChar` run, so this refuses rather than silently accepting a
    /// truncated `9`.
    fn digest(&mut self) -> bool {
        let save = self.pos;

        let format_start = self.pos;
        while self.src.get(self.pos).is_some_and(|&c| is_format_char(c)) {
            self.pos += 1;
        }

        if self.pos == format_start {
            self.pos = save;
            self.fail("expected a markl-id format");

            return false;
        }

        if !self.at('-') {
            self.pos = save;
            self.fail("expected '-' between a markl id's format and data");

            return false;
        }

        self.pos += 1;

        let data_start = self.pos;
        while self.src.get(self.pos).is_some_and(|&c| is_data_char(c)) {
            self.pos += 1;
        }

        if self.pos == data_start {
            self.pos = save;
            self.fail("expected markl-id digest data in the blech32 alphabet");

            return false;
        }

        if is_ident_rune_at(&self.src, self.pos) {
            self.pos = save;
            self.fail("markl-id digest data contains a character outside the blech32 alphabet");

            return false;
        }

        true
    }

    /// `DigestTerm <- '@' Digest` — blob identity, content-addressed exact
    /// match, with a term-initial `@`.
    fn digest_term(&mut self) -> bool {
        let save = self.pos;

        if !self.at('@') {
            self.fail("expected '@' introducing a digest term");
            return false;
        }

        self.pos += 1;

        if !self.digest() {
            self.pos = save;
            return false;
        }

        true
    }

    /// `MarklTerm <- (String / Ident) '@' Digest` — one atomic two-slot
    /// purpose-full markl id.
    ///
    /// Because the purpose slot tries `String` first, a quoted purpose that
    /// itself contains `@` works out naturally: in `"a@b"@blake2b256-xyz` the
    /// `String` consumes `"a@b"` and the join is the SECOND `@`. That is the
    /// quote-aware scan piggy RFC 0011 §2.2 requires — locating the join with the
    /// FIRST `@` would split this id wrongly, and it is the single most
    /// divergence-prone point across the implementations.
    fn markl_term(&mut self) -> bool {
        let save = self.pos;

        if !self.string() {
            self.pos = save;
            if !self.ident_text() {
                self.pos = save;
                return false;
            }
        }

        if !self.at('@') {
            self.pos = save;
            self.fail("expected '@' joining a markl id's purpose and digest");

            return false;
        }

        self.pos += 1;

        if !self.digest() {
            self.pos = save;
            return false;
        }

        true
    }

    // ---- per-line-kind rules ---------------------------------------------

    /// `TypeContent <- MarklTerm / Ident`, the `!` line's content.
    fn type_content(&mut self) -> bool {
        let save = self.pos;
        if self.markl_term() {
            return true;
        }

        self.pos = save;

        self.ident_text()
    }

    /// `BlobContent <- String / Ident`, the `@` line's content: a markl id, or a
    /// file path (quoted if it contains spaces or reserved runes).
    ///
    /// Note this rule does NOT go through `Digest`. The `@` PREFIX is the line's
    /// operator, not a `DigestTerm`'s term-initial `@`, and the value may be a
    /// path rather than a digest, so it stays a plain identifier or string.
    fn blob_content(&mut self) -> bool {
        let save = self.pos;
        if self.string() {
            return true;
        }

        self.pos = save;

        self.ident_text()
    }

    /// `DashContent <- FieldContent / RefContent`, the content of a `-` line and
    /// of its deprecated `<` synonym (RFC 0002 §`<` Deprecation gives them
    /// identical content grammar).
    ///
    /// `FieldContent` is tried first because its branch is the only shape that
    /// can require an `=`: `RefContent`'s bare-term branches would otherwise
    /// short-circuit on the field name and leave `="..."` unconsumed.
    fn dash_content(&mut self) -> bool {
        let save = self.pos;
        if self.field_content() {
            return true;
        }

        self.pos = save;

        self.ref_content()
    }

    /// `RefContent <- GroundTerm` and `GroundTerm <- MarklTerm / String / Ident`.
    fn ref_content(&mut self) -> bool {
        let save = self.pos;
        if self.markl_term() {
            return true;
        }

        self.pos = save;
        if self.string() {
            return true;
        }

        self.pos = save;

        self.ident_text()
    }

    /// ```text
    /// FieldContent <- FieldName SP? '=' SP? FieldRHS
    /// ```
    ///
    /// The optional SP around `=` is RFC 0003 §Canonical emission's spacing.
    fn field_content(&mut self) -> bool {
        let save = self.pos;

        if !self.field_name() {
            self.pos = save;
            return false;
        }

        self.skip_sp_opt();

        if !self.at('=') {
            self.pos = save;
            self.fail("expected '=' in a field predicate");

            return false;
        }

        self.pos += 1;
        self.skip_sp_opt();

        if !self.field_rhs() {
            self.pos = save;
            return false;
        }

        true
    }

    /// `FieldName <- String / Ident` — a flat per-type field namespace, where a
    /// quoted field name is opaque.
    ///
    /// Its body is identical to `blob_content`'s today. That duplication is
    /// deliberate, not an oversight to collapse: this file is organized so every
    /// `.peg` rule has a same-named method, which is what lets a reader diff it
    /// against the grammar rule by rule. These are two distinct rules governing
    /// unrelated things (a field namespace vs. a blob path) that happen to
    /// coincide, and either can diverge without the other.
    fn field_name(&mut self) -> bool {
        let save = self.pos;
        if self.string() {
            return true;
        }

        self.pos = save;

        self.ident_text()
    }

    /// ```text
    /// FieldRHS   <- DigestTerm / FieldValue
    /// FieldValue <- MarklTerm / String / Bareword
    /// ```
    ///
    /// `DigestTerm` comes first so an id-less field lock
    /// (`_base=@blake2b256-…`) is read as a digest term rather than failing into
    /// the value branches.
    fn field_rhs(&mut self) -> bool {
        let save = self.pos;
        if self.digest_term() {
            return true;
        }

        self.pos = save;
        if self.markl_term() {
            return true;
        }

        self.pos = save;
        if self.string() {
            return true;
        }

        self.pos = save;

        // Bareword <- IdentRune+ — the same body as Ident.
        self.ident_text()
    }

    /// `TrailingComment?` where
    ///
    /// ```text
    /// TrailingComment <- SP? '%' (!LF .)*
    /// ```
    ///
    /// The SP is OPTIONAL, not required (ruling 2026-07-20): `%` is `Reserved`
    /// and so self-delimiting against every content production, which makes a
    /// glued `md%comment` and a spaced `md % comment` parse identically. The
    /// `SP?` still has to exist to consume the space in the spaced form — in a
    /// PEG, dropping it would FORBID the space rather than permit glue.
    fn trailing_comment_opt(&mut self) {
        let save = self.pos;

        self.skip_sp_opt();

        if !self.at('%') {
            self.pos = save;
            return;
        }

        self.pos += 1;
        self.free_text();
    }

    /// `FreeText <- (!LF .)*`, which is also `OpaqueComment`'s body and a
    /// trailing comment's tail. LF is RFC 0001's own line terminator, so this
    /// stops at one rather than consuming it.
    fn free_text(&mut self) {
        while self.src.get(self.pos).is_some_and(|&c| c != '\n') {
            self.pos += 1;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The shapes RFC 0002/0003 admit. Deliberately the same table the Go side
    /// pins in `TestParseContent_Accepts`, so a divergence between the two
    /// parsers shows up as one of them failing a case the other passes.
    #[test]
    fn accepts_conforming_content() {
        for (name, prefix, content) in [
            ("type/bare-ident", Prefix::Type, "md"),
            ("type/markl-term", Prefix::Type, "md@blake2b256-acd"),
            (
                "type/glued-trailing-comment",
                Prefix::Type,
                "md%glued comment",
            ),
            ("type/spaced-trailing-comment", Prefix::Type, "md % spaced"),
            // BlobContent is String / Ident and does NOT go through Digest, so a
            // 'b' that Digest would reject is fine in this position.
            ("blob/ident", Prefix::Blob, "blake2b256-abc"),
            ("blob/quoted-path", Prefix::Blob, "'pictures/a photo.png'"),
            (
                "dash/reference-markl-term",
                Prefix::TagRef,
                "one/uno@blake2b256-def",
            ),
            (
                "dash/field-quoted-value",
                Prefix::TagRef,
                r#"due="2026-08-01""#,
            ),
            (
                "dash/field-spaced-equals",
                Prefix::TagRef,
                r#"due = "2026-08-01""#,
            ),
            ("dash/field-bareword-value", Prefix::TagRef, "state=open"),
            (
                "dash/id-less-field-lock",
                Prefix::TagRef,
                "_base=@blake2b256-jkl",
            ),
            (
                "dash/typed-edge",
                Prefix::TagRef,
                "blocks=task/other@blake2b256-ghj",
            ),
            (
                "dash/quoted-purpose",
                Prefix::TagRef,
                r#""my thing"@blake2b256-xyz"#,
            ),
            (
                "dash/quoted-field-name",
                Prefix::TagRef,
                r#""odd name"=value"#,
            ),
            (
                "dash/key-material-shape",
                Prefix::TagRef,
                "piggy-piv_auth-v1@ssh_ecdsa_nistp256_pub-qqxyz",
            ),
            (
                "dash/quoting-composes-with-comment",
                Prefix::TagRef,
                r#"note="50% done" % real comment"#,
            ),
            // A quoted purpose CONTAINING the join rune: the join is the SECOND
            // '@', which only falls out of a quote-aware purpose match.
            (
                "dash/quoted-purpose-with-join-rune",
                Prefix::TagRef,
                r#""a@b"@blake2b256-xyz"#,
            ),
            // An interior sigil is identifier content because what follows it
            // is — the &IdentRune lookahead's positive case.
            (
                "dash/interior-sigil-is-ident",
                Prefix::TagRef,
                "caldav:fastmail",
            ),
            (
                "angle/field-predicate",
                Prefix::ObjectRef,
                "blocks=other/task@blake2b256-def",
            ),
            (
                "free-text/prose-with-reserved-runes",
                Prefix::Description,
                r#"100% of "a@b" [done]"#,
            ),
            ("comment/opaque", Prefix::Comment, r#"anything at all @ ""#),
            ("free-text/empty", Prefix::Description, ""),
        ] {
            assert_eq!(
                parse_content(prefix, content),
                Ok(()),
                "{name}: expected {content:?} to parse"
            );
        }
    }

    /// What the charset-strict digest and the per-prefix productions refuse. The
    /// digest cases are hyphence#11's behavior change: every one of them parsed
    /// under the former permissive `'@' Ident` digest slot.
    #[test]
    fn rejects_malformed_content() {
        for (name, prefix, content) in [
            (
                "digest/non-blech32-in-type",
                Prefix::Type,
                "md@blake2b256-9bt3",
            ),
            (
                "digest/non-blech32-in-field",
                Prefix::TagRef,
                "pinned=other/thing@blake2b256-9bt3",
            ),
            (
                "digest/non-blech32-in-digest-term",
                Prefix::TagRef,
                "_base=@blake2b256-9bt3",
            ),
            // blech32 is lowercase-only (piggy RFC 0011 §3.5).
            ("digest/uppercase-data", Prefix::Type, "md@BLAKE2B256-9FT3"),
            ("digest/missing-separator", Prefix::Type, "md@blake2b256"),
            ("digest/empty-data", Prefix::Type, "md@blake2b256-"),
            ("digest/missing-digest", Prefix::Type, "md@"),
            ("digest/missing-format", Prefix::TagRef, "_base=@-9ft3x"),
            ("string/unterminated", Prefix::TagRef, r#""unterminated"#),
            ("type/unterminated-quote", Prefix::Type, r#""oops"#),
            // Ident is IdentRune+ — one or more.
            ("type/empty", Prefix::Type, ""),
            ("dash/empty", Prefix::TagRef, ""),
            ("blob/empty", Prefix::Blob, ""),
            // Whitespace is not identifier content, so the second word is
            // unconsumed trailing input.
            ("dash/unquoted-space", Prefix::TagRef, "foo bar"),
            ("dash/reserved-rune-unquoted", Prefix::TagRef, "a,b"),
        ] {
            let got = parse_content(prefix, content);
            let err = got.expect_err(name);

            assert_eq!(err.prefix, prefix, "{name}: prefix mismatch");
            assert!(!err.message.is_empty(), "{name}: empty diagnostic");
        }
    }

    /// The recursive `&IdentRune` lookahead: a sigil rune is identifier content
    /// only when what follows it is, so a term-final sigil (or a run of them) is
    /// not.
    #[test]
    fn strict_sigil_rule() {
        for (name, input, at, want) in [
            ("interior-sigil-followed-by-ident", "a:b", 1, true),
            ("term-final-sigil", "a:", 1, false),
            ("trailing-sigil-run-bottoms-out", "a::", 1, false),
            ("sigil-run-then-ident", "a::b", 1, true),
            ("hyphen-is-unconditional", "a-", 1, true),
            ("slash-is-unconditional", "a/", 1, true),
            ("reserved-rune", "a@b", 1, false),
            ("whitespace", "a b", 1, false),
            ("plain-rune", "ab", 1, true),
            ("out-of-range", "a", 1, false),
        ] {
            let chars: Vec<char> = input.chars().collect();
            assert_eq!(
                is_ident_rune_at(&chars, at),
                want,
                "{name}: is_ident_rune_at({input:?}, {at})"
            );
        }
    }

    /// The four characters bech32 drops as visually ambiguous. These are exactly
    /// what separates the strict digest from the former permissive identifier
    /// slot, so a regression here would silently restore the old laxness.
    #[test]
    fn data_char_omits_ambiguous_runes() {
        for c in ['b', 'i', 'o', '1'] {
            assert!(!is_data_char(c), "{c:?} is outside blech32");
        }

        const BLECH32: &str = "qpzry9x8gf2tvdw0s3jn54khce6mua7l";
        for c in BLECH32.chars() {
            assert!(is_data_char(c), "{c:?} is in blech32");
        }

        assert_eq!(BLECH32.chars().count(), 32);
    }
}
