//! The in-memory model of a hyphence document: a metadata section plus an
//! optional opaque body.

/// The single-character prefix that keys each metadata line (RFC 0001
/// §Metadata Lines). The prefix selects the line's role; the content after the
/// prefix-and-space is otherwise opaque at the envelope level.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Prefix {
    /// `!` — object type identifier; decides how the body is interpreted.
    Type,
    /// `@` — blob reference (markl-id or path); an alternative to an inline body.
    Blob,
    /// `#` — free-text description.
    Description,
    /// `-` — tag or object reference (bare value = tag, value with `/` = ref).
    TagRef,
    /// `<` — explicit object reference.
    ObjectRef,
    /// `%` — comment; preserved across round-trips, entangled with the next
    /// non-comment line.
    Comment,
}

impl Prefix {
    /// Map a prefix byte to its `Prefix`, or `None` if it is not one of `!@#-<%`.
    pub fn from_byte(b: u8) -> Option<Prefix> {
        Some(match b {
            b'!' => Prefix::Type,
            b'@' => Prefix::Blob,
            b'#' => Prefix::Description,
            b'-' => Prefix::TagRef,
            b'<' => Prefix::ObjectRef,
            b'%' => Prefix::Comment,
            _ => return None,
        })
    }

    /// The prefix's byte, for emitting.
    pub fn byte(self) -> u8 {
        match self {
            Prefix::Type => b'!',
            Prefix::Blob => b'@',
            Prefix::Description => b'#',
            Prefix::TagRef => b'-',
            Prefix::ObjectRef => b'<',
            Prefix::Comment => b'%',
        }
    }

    /// Canonical emit-order rank (RFC 0001 §Encoder Behavior): descriptions, then
    /// object references / tags, then the blob line, then the type line. The
    /// finer locked/aliased/bare sub-ordering of references is collapsed into one
    /// rank here (it needs lock parsing this envelope layer does not do); source
    /// order within a rank is preserved by a stable sort. `Comment` never appears
    /// as a standalone metadata line (comments live in `leading_comments`).
    pub(crate) fn order_rank(self) -> u8 {
        match self {
            Prefix::Description => 0,
            Prefix::ObjectRef | Prefix::TagRef => 1,
            Prefix::Blob => 2,
            Prefix::Type => 3,
            Prefix::Comment => 4,
        }
    }
}

/// One non-comment metadata line, with any `%` comment lines that immediately
/// preceded it carried along (RFC 0001: comments are entangled with the next
/// non-comment line, so reordering moves them too).
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct MetadataLine {
    pub prefix: Prefix,
    pub value: String,
    pub leading_comments: Vec<String>,
}

/// A parsed hyphence document: the metadata lines, any trailing comments that
/// followed the last non-comment line, and the optional body bytes. The body is
/// opaque (`Vec<u8>`) — its interpretation is the concern of the `!` type line.
#[derive(Clone, Debug, PartialEq, Eq, Default)]
pub struct Document {
    pub metadata: Vec<MetadataLine>,
    pub trailing_comments: Vec<String>,
    pub body: Option<Vec<u8>>,
}

impl Document {
    /// The common constructor: a single `!` type line plus an opaque body. This is
    /// the shape a versioned-blob writer uses — the version rides in `type_str`
    /// (e.g. `mephisto-population-controller-schema_v1`).
    pub fn with_type_and_body(type_str: impl Into<String>, body: impl Into<Vec<u8>>) -> Document {
        Document {
            metadata: vec![MetadataLine {
                prefix: Prefix::Type,
                value: type_str.into(),
                leading_comments: Vec::new(),
            }],
            trailing_comments: Vec::new(),
            body: Some(body.into()),
        }
    }

    /// The value of the `!` type line, if present — the version-bearing field a
    /// reader inspects before deciding how (or whether) to decode the body.
    pub fn type_line(&self) -> Option<&str> {
        self.line_value(Prefix::Type)
    }

    /// The value of the `@` blob-reference line, if present.
    pub fn blob_ref(&self) -> Option<&str> {
        self.line_value(Prefix::Blob)
    }

    /// All `#` description values, in document order.
    pub fn descriptions(&self) -> impl Iterator<Item = &str> {
        self.metadata
            .iter()
            .filter(|l| l.prefix == Prefix::Description)
            .map(|l| l.value.as_str())
    }

    fn line_value(&self, prefix: Prefix) -> Option<&str> {
        self.metadata
            .iter()
            .find(|l| l.prefix == prefix)
            .map(|l| l.value.as_str())
    }
}
