//! The typed head: a hyphence document's metadata section read into typed lines
//! (RFC 0002 as amended by RFC 0003), and the canonical encoder that writes it
//! back.
//!
//! [`Document`] keeps every line's content as an opaque string, which is all
//! RFC 0001 needs. [`Head`] is the layer above it: each line parsed by the
//! production its prefix selects, every quoted string unescaped, every markl id
//! split into its purpose and digest slots. A consumer reads a head, reorders or
//! edits its lines, and encodes it; two heads with equal typed content encode to
//! equal bytes.
//!
//! # What stays out
//!
//! - **Markl decoding.** A [`Digest`] is TEXT: hyphence checks the lexeme
//!   (format runes, a blech32-alphabet payload) and never decodes blech32 or
//!   checks a length or checksum. That is piggy-markl's job, and this crate
//!   stays dependency-free.
//! - **The body.** Bytes, untouched. Its meaning is the `!` type's concern.
//! - **Field tables.** Which fields a type requires, their order, set
//!   semantics, refusals: the consumer's. Hyphence orders lines by prefix
//!   rank only, keeping the consumer's order within a rank.
//!
//! # Canonical emission
//!
//! [`Head::to_document`] spells every line canonically and [`Document::encode`]
//! then emits the envelope:
//!
//! - prefix order `#`, `-`, `@`, `!` (RFC 0002 §`<` Deprecation), stable, so
//!   lines of one rank keep the order they have in [`Head::lines`];
//! - `<` is never emitted: a `<` line reads as a `-` line (RFC 0002), and is
//!   written `-`;
//! - field lines spaced, `- key = value` (RFC 0003 §Canonical emission:
//!   spacing);
//! - a text (field name, value, tag or reference, blob path) bare when it is a
//!   valid `Ident`, otherwise double-quoted with [`quote`]'s escapes;
//! - a markl id's purpose bare when it fits markl's own `PurposeBare`
//!   (`[a-zA-Z0-9_/-]+`, piggy `marklid.peg`), otherwise quoted, joined by `@`
//!   right after the closing quote; a purpose-less markl id in a field value is
//!   the id-less `DigestTerm`, `@format-data`;
//! - comments KEPT: `%` lines before the line they are entangled with, and a
//!   trailing comment as ` %text` after the content (RFC 0001: comments are
//!   preserved on round-trip; RFC 0002: content-preserving encoders MUST keep
//!   trailing comments). A consumer that wants them gone removes them from the
//!   model first.

use std::fmt;

use crate::content::{ContentError, is_bare_ident, is_digest_parts, parse_line};
use crate::document::{Document, MetadataLine, Prefix};
use crate::error::Error;

/// A markl-id digest slot, `format-data`, as text: `Format '-' DataChar+`.
///
/// Construction checks the lexeme only — format runes `[a-zA-Z0-9_]`, a
/// non-empty payload in the blech32 alphabet. It is length-agnostic and never
/// decodes the payload.
#[derive(Clone, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct Digest {
    format: String,
    data: String,
}

impl Digest {
    /// A digest from its format id and its data payload.
    ///
    /// # Errors
    ///
    /// [`InvalidDigest`] when either part is outside its charset or empty.
    pub fn new(
        format: impl Into<String>,
        data: impl Into<String>,
    ) -> Result<Digest, InvalidDigest> {
        let (format, data) = (format.into(), data.into());
        if !is_digest_parts(&format, &data) {
            return Err(InvalidDigest { format, data });
        }

        Ok(Digest { format, data })
    }

    /// Parse the `format-data` spelling. The split is at the first `-`, since
    /// a format id never contains one.
    ///
    /// # Errors
    ///
    /// [`InvalidDigest`] when there is no `-` or either part is malformed.
    pub fn parse(s: &str) -> Result<Digest, InvalidDigest> {
        let (format, data) = s.split_once('-').unwrap_or((s, ""));
        Digest::new(format, data)
    }

    pub(crate) fn from_checked_parts(format: String, data: String) -> Digest {
        Digest { format, data }
    }

    /// The format id, e.g. `blake2b256`.
    pub fn format(&self) -> &str {
        &self.format
    }

    /// The blech32-alphabet payload, undecoded.
    pub fn data(&self) -> &str {
        &self.data
    }
}

impl fmt::Display for Digest {
    /// The `format-data` spelling.
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}-{}", self.format, self.data)
    }
}

/// A format id and payload that do not form a `Digest` slot.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct InvalidDigest {
    pub format: String,
    pub data: String,
}

impl fmt::Display for InvalidDigest {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "not a markl-id digest: format {:?}, data {:?} (format must be [a-zA-Z0-9_]+, data blech32)",
            self.format, self.data
        )
    }
}

impl std::error::Error for InvalidDigest {}

/// A markl id as text: an optional purpose joined to a digest
/// (`[purpose@]format-data`, piggy RFC 0011 §2).
#[derive(Clone, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct MarklId {
    /// The purpose, UNESCAPED (`"a@b"@…` reads as `a@b`). `None` is a
    /// purpose-less id, which hyphence content admits only as a field's
    /// id-less `DigestTerm` (`- _base = @blake2b256-…`).
    pub purpose: Option<String>,
    pub digest: Digest,
}

/// A field value, or the term of a tag/reference line.
///
/// A bare word and a quoted string are ONE kind here: both are text, and the
/// encoder chooses the spelling (bare when the grammar allows it). So `"open"`
/// and `open` are equal content.
#[derive(Clone, Debug, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub enum Value {
    Markl(MarklId),
    Text(String),
}

/// One metadata line's typed content.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Line {
    /// `#` free text, verbatim (no trimming, no merging).
    Description(String),
    /// `- name = value` (RFC 0002 `FieldContent`). The name is unescaped.
    Field { name: String, value: Value },
    /// `- term`: a tag or a reference (RFC 0002 deleted the `/`-shape rule, so
    /// hyphence does not tell them apart). A `Markl` term is a locked
    /// reference and always has a purpose.
    Term(Value),
    /// `@` blob reference: a markl id or a path, as text (`BlobContent`).
    Blob(String),
    /// `!` type, optionally locked (`! md@blake2b256-…`).
    Type { name: String, lock: Option<Digest> },
}

/// A typed line with the comments entangled with it.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct HeadLine {
    pub line: Line,
    /// `%` lines immediately before this one (RFC 0001: entangled with the
    /// next non-comment line, so they move with it).
    pub leading_comments: Vec<String>,
    /// The text after a trailing `%` on this line (RFC 0002), verbatim.
    pub trailing_comment: Option<String>,
}

impl HeadLine {
    /// A line with no comments.
    pub fn new(line: Line) -> HeadLine {
        HeadLine {
            line,
            leading_comments: Vec::new(),
            trailing_comment: None,
        }
    }
}

impl From<Line> for HeadLine {
    fn from(line: Line) -> HeadLine {
        HeadLine::new(line)
    }
}

/// A document's metadata section as typed lines.
#[derive(Clone, Debug, PartialEq, Eq, Default)]
pub struct Head {
    /// Lines in source order. The encoder sorts by prefix rank only, so this
    /// order is kept within a rank: a consumer orders its fields by ordering
    /// this vector.
    pub lines: Vec<HeadLine>,
    /// `%` lines after the last non-comment line.
    pub trailing_comments: Vec<String>,
}

/// Why bytes or a [`Document`] did not read as a typed head.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum HeadError {
    /// The RFC 0001 envelope did not decode.
    Envelope(Error),
    /// A line's content does not parse under its prefix's production.
    Content {
        /// The index into [`Document::metadata`] (comments not counted).
        line: usize,
        error: ContentError,
    },
}

impl fmt::Display for HeadError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            HeadError::Envelope(e) => e.fmt(f),
            HeadError::Content { line, error } => write!(f, "metadata line {line}: {error}"),
        }
    }
}

impl std::error::Error for HeadError {}

impl From<Error> for HeadError {
    fn from(e: Error) -> HeadError {
        HeadError::Envelope(e)
    }
}

/// Why a [`Head`] has no canonical spelling.
#[derive(Clone, Debug, PartialEq, Eq)]
pub enum EncodeError {
    /// An unlocked type name that is not an `Ident`: `TypeContent` admits a
    /// quoted name only as a locked markl id's purpose.
    TypeNameNotIdent(String),
    /// A tag/reference term that is a markl id with no purpose: `GroundTerm`
    /// has no `DigestTerm` alternative.
    PurposelessTerm(Digest),
    /// A description or comment containing a line feed, which would end the
    /// line early.
    LineFeed(String),
}

impl fmt::Display for EncodeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            EncodeError::TypeNameNotIdent(name) => {
                write!(f, "unlocked type name {name:?} is not an identifier")
            }
            EncodeError::PurposelessTerm(digest) => write!(
                f,
                "a tag/reference line cannot hold the purpose-less markl id @{digest}"
            ),
            EncodeError::LineFeed(text) => write!(f, "line feed inside free text {text:?}"),
        }
    }
}

impl std::error::Error for EncodeError {}

impl Head {
    /// Read every metadata line of `doc` as typed content. `<` lines read as
    /// `-` lines. Fails on the first line the content grammar refuses,
    /// including the retired RFC 0002 spaced-lock spelling.
    ///
    /// # Errors
    ///
    /// [`HeadError::Content`] naming the line.
    pub fn from_document(doc: &Document) -> Result<Head, HeadError> {
        let mut lines = Vec::with_capacity(doc.metadata.len());

        for (index, metadata) in doc.metadata.iter().enumerate() {
            let parsed = parse_line(metadata.prefix, &metadata.value)
                .map_err(|error| HeadError::Content { line: index, error })?;

            // The envelope never yields a `%` metadata line (comments ride on
            // leading_comments), so a typed line is always present here.
            if let Some((line, trailing_comment)) = parsed {
                lines.push(HeadLine {
                    line,
                    leading_comments: metadata.leading_comments.clone(),
                    trailing_comment,
                });
            }
        }

        Ok(Head {
            lines,
            trailing_comments: doc.trailing_comments.clone(),
        })
    }

    /// Decode bytes into a typed head and the untouched body.
    ///
    /// # Errors
    ///
    /// [`HeadError::Envelope`] or [`HeadError::Content`].
    pub fn decode(input: &[u8]) -> Result<(Head, Option<Vec<u8>>), HeadError> {
        let doc = Document::decode(input)?;
        let head = Head::from_document(&doc)?;

        Ok((head, doc.body))
    }

    /// The canonical [`Document`] for this head and `body`: every line spelled
    /// canonically. [`Document::encode`] then orders the lines and writes the
    /// envelope.
    ///
    /// # Errors
    ///
    /// [`EncodeError`] when a line has no spelling.
    pub fn to_document(&self, body: Option<Vec<u8>>) -> Result<Document, EncodeError> {
        let metadata = self
            .lines
            .iter()
            .map(|line| {
                let (prefix, mut value) = spell_line(&line.line)?;

                if let Some(comment) = &line.trailing_comment {
                    check_no_line_feed(comment)?;
                    value.push_str(" %");
                    value.push_str(comment);
                }

                for comment in &line.leading_comments {
                    check_no_line_feed(comment)?;
                }

                Ok(MetadataLine {
                    prefix,
                    value,
                    leading_comments: line.leading_comments.clone(),
                })
            })
            .collect::<Result<Vec<_>, EncodeError>>()?;

        for comment in &self.trailing_comments {
            check_no_line_feed(comment)?;
        }

        Ok(Document {
            metadata,
            trailing_comments: self.trailing_comments.clone(),
            body,
        })
    }

    /// The canonical bytes for this head and `body`.
    ///
    /// `Some(&[])` is an empty body, which RFC 0001 writes as a separator line
    /// with nothing after it; pass `None` for no body.
    ///
    /// # Errors
    ///
    /// [`EncodeError`] when a line has no spelling.
    pub fn encode(&self, body: Option<&[u8]>) -> Result<Vec<u8>, EncodeError> {
        Ok(self.to_document(body.map(<[u8]>::to_vec))?.encode())
    }

    /// The `- name = value` lines, in order.
    pub fn fields(&self) -> impl Iterator<Item = (&str, &Value)> {
        self.lines.iter().filter_map(|l| match &l.line {
            Line::Field { name, value } => Some((name.as_str(), value)),
            _ => None,
        })
    }

    /// The first `!` type line's name and lock, if present.
    pub fn type_line(&self) -> Option<(&str, Option<&Digest>)> {
        self.lines.iter().find_map(|l| match &l.line {
            Line::Type { name, lock } => Some((name.as_str(), lock.as_ref())),
            _ => None,
        })
    }
}

fn check_no_line_feed(text: &str) -> Result<(), EncodeError> {
    if text.contains('\n') {
        return Err(EncodeError::LineFeed(text.to_owned()));
    }

    Ok(())
}

/// One line's prefix and canonical content (without any trailing comment).
fn spell_line(line: &Line) -> Result<(Prefix, String), EncodeError> {
    Ok(match line {
        Line::Description(text) => {
            check_no_line_feed(text)?;
            (Prefix::Description, text.clone())
        }
        Line::Field { name, value } => (
            Prefix::TagRef,
            format!("{} = {}", spell_text(name), spell_value(value)),
        ),
        Line::Term(Value::Markl(MarklId {
            purpose: None,
            digest,
        })) => return Err(EncodeError::PurposelessTerm(digest.clone())),
        Line::Term(value) => (Prefix::TagRef, spell_value(value)),
        Line::Blob(path) => (Prefix::Blob, spell_text(path)),
        Line::Type { name, lock: None } => {
            if !is_bare_ident(name) {
                return Err(EncodeError::TypeNameNotIdent(name.clone()));
            }
            (Prefix::Type, name.clone())
        }
        Line::Type {
            name,
            lock: Some(digest),
        } => (Prefix::Type, format!("{}@{digest}", spell_purpose(name))),
    })
}

fn spell_value(value: &Value) -> String {
    match value {
        Value::Text(text) => spell_text(text),
        Value::Markl(MarklId {
            purpose: None,
            digest,
        }) => format!("@{digest}"),
        Value::Markl(MarklId {
            purpose: Some(purpose),
            digest,
        }) => format!("{}@{digest}", spell_purpose(purpose)),
    }
}

/// A text bare when it is a valid `Ident`, else quoted.
fn spell_text(text: &str) -> String {
    if is_bare_ident(text) {
        text.to_owned()
    } else {
        quote(text)
    }
}

/// A markl purpose bare when it fits marklid.peg's `PurposeBare`
/// (`[a-zA-Z0-9_/-]+`), else quoted — the same rule as piggy-markl's
/// `spell_purpose`.
fn spell_purpose(purpose: &str) -> String {
    let bare = !purpose.is_empty()
        && purpose
            .chars()
            .all(|c| c.is_ascii_alphanumeric() || matches!(c, '_' | '/' | '-'));

    if bare {
        purpose.to_owned()
    } else {
        quote(purpose)
    }
}

/// Double-quote `text` with the Doddish escapes piggy-markl's `quote_purpose`
/// writes: `\\ \" \n \t \r \a \b \f \v`; every other char is itself.
pub fn quote(text: &str) -> String {
    let mut out = String::with_capacity(text.len() + 2);
    out.push('"');

    for c in text.chars() {
        match c {
            '\\' => out.push_str("\\\\"),
            '"' => out.push_str("\\\""),
            '\n' => out.push_str("\\n"),
            '\t' => out.push_str("\\t"),
            '\r' => out.push_str("\\r"),
            '\u{7}' => out.push_str("\\a"),
            '\u{8}' => out.push_str("\\b"),
            '\u{c}' => out.push_str("\\f"),
            '\u{b}' => out.push_str("\\v"),
            other => out.push(other),
        }
    }

    out.push('"');
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    fn digest(s: &str) -> Digest {
        Digest::parse(s).expect("test digest")
    }

    fn canonical(input: &str) -> String {
        let (head, body) = Head::decode(input.as_bytes()).expect("decodes");
        String::from_utf8(head.encode(body.as_deref()).expect("encodes")).expect("utf8")
    }

    #[test]
    fn reads_every_line_kind() {
        let input = "---\n\
                     # a description\n\
                     % entangled\n\
                     - kind = root\n\
                     - \"odd name\"=\"a \\\"q\\\" b\"\n\
                     - key=piggy-piv_auth-v1@ssh_ecdsa_nistp256_pub-qqxyz\n\
                     - _base=@blake2b256-jkl\n\
                     - tag\n\
                     < one/uno@blake2b256-def % pinned\n\
                     @ 'a photo.png'\n\
                     ! md@blake2b256-acd\n\
                     % tail\n\
                     ---\n";
        let (head, body) = Head::decode(input.as_bytes()).expect("decodes");

        assert_eq!(body, None);
        assert_eq!(
            head.lines
                .iter()
                .map(|l| l.line.clone())
                .collect::<Vec<_>>(),
            vec![
                Line::Description("a description".into()),
                Line::Field {
                    name: "kind".into(),
                    value: Value::Text("root".into())
                },
                Line::Field {
                    name: "odd name".into(),
                    value: Value::Text("a \"q\" b".into())
                },
                Line::Field {
                    name: "key".into(),
                    value: Value::Markl(MarklId {
                        purpose: Some("piggy-piv_auth-v1".into()),
                        digest: digest("ssh_ecdsa_nistp256_pub-qqxyz"),
                    })
                },
                Line::Field {
                    name: "_base".into(),
                    value: Value::Markl(MarklId {
                        purpose: None,
                        digest: digest("blake2b256-jkl"),
                    })
                },
                Line::Term(Value::Text("tag".into())),
                Line::Term(Value::Markl(MarklId {
                    purpose: Some("one/uno".into()),
                    digest: digest("blake2b256-def"),
                })),
                Line::Blob("a photo.png".into()),
                Line::Type {
                    name: "md".into(),
                    lock: Some(digest("blake2b256-acd")),
                },
            ]
        );
        // A `%` line's text is what follows the prefix and its space; a
        // trailing comment's is everything after the `%`.
        assert_eq!(head.lines[1].leading_comments, vec!["entangled".to_owned()]);
        assert_eq!(head.lines[6].trailing_comment.as_deref(), Some(" pinned"));
        assert_eq!(head.trailing_comments, vec!["tail".to_owned()]);
    }

    #[test]
    fn canonical_spelling() {
        for (name, input, want) in [
            (
                "unspaced field re-emitted spaced",
                "---\n- due=\"2026-08-01\"\n---\n",
                "---\n- due = 2026-08-01\n---\n",
            ),
            (
                "quoted value that can be bare",
                "---\n- state=\"open\"\n---\n",
                "---\n- state = open\n---\n",
            ),
            (
                "single quotes re-emitted double",
                "---\n- note='a b'\n---\n",
                "---\n- note = \"a b\"\n---\n",
            ),
            (
                "escapes round-trip",
                "---\n- note=\"a\\\"b\\\\c\\td\"\n---\n",
                "---\n- note = \"a\\\"b\\\\c\\td\"\n---\n",
            ),
            (
                "a line feed in a value never breaks the line",
                "---\n- note=\"a\\nb\"\n---\n",
                "---\n- note = \"a\\nb\"\n---\n",
            ),
            (
                "unknown escape collapses",
                "---\n- note=\"a\\qb c\"\n---\n",
                "---\n- note = \"aqb c\"\n---\n",
            ),
            (
                "quoted purpose with the join rune",
                "---\n- \"a@b\"@blake2b256-xyz\n---\n",
                "---\n- \"a@b\"@blake2b256-xyz\n---\n",
            ),
            (
                "quoted purpose that can be bare",
                "---\n- k=\"md\"@blake2b256-acd\n---\n",
                "---\n- k = md@blake2b256-acd\n---\n",
            ),
            (
                "ident purpose outside PurposeBare gets quoted",
                "---\n- k=caldav:x@blake2b256-acd\n---\n",
                "---\n- k = \"caldav:x\"@blake2b256-acd\n---\n",
            ),
            (
                "angle line emitted as dash",
                "---\n< blocks=other/task@blake2b256-ghj\n---\n",
                "---\n- blocks = other/task@blake2b256-ghj\n---\n",
            ),
            (
                "prefix order and comments kept",
                "---\n! md\n% about the field\n- k=v%glued\n# d\n---\n\nbody\n",
                "---\n# d\n% about the field\n- k = v %glued\n! md\n---\n\nbody\n",
            ),
            (
                "term-final sigil forces quoting",
                "---\n- \"todo:\"\n---\n",
                "---\n- \"todo:\"\n---\n",
            ),
            (
                "empty text quoted",
                "---\n- k=\"\"\n---\n",
                "---\n- k = \"\"\n---\n",
            ),
        ] {
            assert_eq!(canonical(input), want, "{name}");
            assert_eq!(canonical(want), want, "{name}: not idempotent");
        }
    }

    #[test]
    fn equal_content_equal_bytes() {
        let a = canonical("---\n! t\n- k=\"v\"\n% c\n- j='w x'\n---\n");
        let b = canonical("---\n- k = v\n% c\n- j = \"w x\"\n! t\n---\n");
        assert_eq!(a, b);
    }

    #[test]
    fn consumer_orders_within_a_rank() {
        let (mut head, _) = Head::decode(b"---\n! t\n- b = 2\n- a = 1\n---\n").unwrap();
        head.lines.sort_by(|x, y| match (&x.line, &y.line) {
            (Line::Field { name: n, .. }, Line::Field { name: m, .. }) => n.cmp(m),
            _ => std::cmp::Ordering::Equal,
        });

        assert_eq!(
            head.encode(None).unwrap(),
            b"---\n- a = 1\n- b = 2\n! t\n---\n"
        );
    }

    #[test]
    fn refuses_what_the_grammar_refuses() {
        for input in [
            "---\n! md @blake2b256-acd\n---\n",
            "---\n- tag(1)\n---\n",
            "---\n! md@blake2b256-9bt3\n---\n",
        ] {
            assert!(
                matches!(
                    Head::decode(input.as_bytes()),
                    Err(HeadError::Content { .. })
                ),
                "{input:?}"
            );
        }
    }

    #[test]
    fn encode_errors() {
        let type_with_space = Head {
            lines: vec![HeadLine::new(Line::Type {
                name: "a b".into(),
                lock: None,
            })],
            ..Head::default()
        };
        assert!(matches!(
            type_with_space.encode(None),
            Err(EncodeError::TypeNameNotIdent(_))
        ));

        let purposeless_term = Head {
            lines: vec![HeadLine::new(Line::Term(Value::Markl(MarklId {
                purpose: None,
                digest: digest("blake2b256-acd"),
            })))],
            ..Head::default()
        };
        assert!(matches!(
            purposeless_term.encode(None),
            Err(EncodeError::PurposelessTerm(_))
        ));

        let multi_line_description = Head {
            lines: vec![HeadLine::new(Line::Description("a\nb".into()))],
            ..Head::default()
        };
        assert!(matches!(
            multi_line_description.encode(None),
            Err(EncodeError::LineFeed(_))
        ));
    }

    #[test]
    fn digest_checks_the_lexeme_only() {
        assert!(Digest::parse("blake2b256-9ft3x").is_ok());
        assert!(Digest::parse("blake2b256-9bt3").is_err());
        assert!(Digest::parse("blake2b256-").is_err());
        assert!(Digest::parse("blake2b256").is_err());
        assert!(Digest::new("", "qq").is_err());
    }
}
