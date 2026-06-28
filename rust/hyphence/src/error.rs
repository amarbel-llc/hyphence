//! Decode errors, one variant per RFC 0001 rejection rule.

use std::fmt;

/// A reason a byte stream is not a valid hyphence document. Each variant maps to
/// a normative rejection in RFC 0001; callers can match on them (e.g. to tell the
/// strict missing-separator case apart from a malformed line).
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Error {
    /// A body followed the closing boundary without the required blank separator
    /// line (RFC 0001 §Body Separator). The strict default; motivated by dodder#41,
    /// where lenient parsing silently produced empty-digest corruption.
    MissingSeparator,
    /// A metadata line began with a character that is not one of `!@#-<%`
    /// (RFC 0001 §Metadata Lines / §Decoder Behavior).
    InvalidPrefix,
    /// A metadata line did not match `<prefix><space><content>` — typically a
    /// prefix character with no following space.
    MalformedLine,
    /// The document carried both an `@` blob-reference line and a body section,
    /// which RFC 0001 §Metadata Lines says decoders SHOULD reject.
    InlineBodyWithAtReference,
    /// End of input was reached inside the metadata section: an opening boundary
    /// with no matching closing boundary.
    UnterminatedMetadata,
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let msg = match self {
            Error::MissingSeparator => {
                "missing blank line after closing boundary --- before the body"
            }
            Error::InvalidPrefix => "metadata line has an invalid prefix (not one of !@#-<%)",
            Error::MalformedLine => "malformed metadata line (expected `<prefix> <content>`)",
            Error::InlineBodyWithAtReference => {
                "an `@` blob reference is not allowed when a body section follows"
            }
            Error::UnterminatedMetadata => "unterminated metadata section (no closing boundary)",
        };
        f.write_str(msg)
    }
}

impl std::error::Error for Error {}
