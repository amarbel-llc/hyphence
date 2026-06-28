//! Strict RFC 0001 decoder: bytes -> `Document`.

use crate::document::{Document, MetadataLine, Prefix};
use crate::error::Error;

/// The boundary byte sequence: exactly three hyphen-minus then one line feed.
const BOUNDARY: &[u8] = b"---\n";

impl Document {
    /// Parse a hyphence document from raw bytes.
    ///
    /// Strict per RFC 0001:
    /// - the opening boundary is exactly `---\n`; if it is absent the whole input
    ///   is taken as the body with no metadata,
    /// - each metadata line is `<prefix><space><content>` (an unknown prefix is
    ///   [`Error::InvalidPrefix`]; a prefix with no following space is
    ///   [`Error::MalformedLine`]),
    /// - a body is present only when a single blank separator line follows the
    ///   closing boundary; any other non-EOF byte there is [`Error::MissingSeparator`],
    /// - an `@` blob line together with a body is [`Error::InlineBodyWithAtReference`],
    /// - EOF inside the metadata section is [`Error::UnterminatedMetadata`].
    pub fn decode(input: &[u8]) -> Result<Document, Error> {
        // No opening boundary: the entire input is an unstructured body.
        if !boundary_at(input, 0) {
            return Ok(Document {
                metadata: Vec::new(),
                trailing_comments: Vec::new(),
                body: (!input.is_empty()).then(|| input.to_vec()),
            });
        }

        let mut cursor = BOUNDARY.len();
        let mut metadata: Vec<MetadataLine> = Vec::new();
        let mut pending_comments: Vec<String> = Vec::new();

        loop {
            if boundary_at(input, cursor) {
                cursor += BOUNDARY.len();
                break;
            }
            // A line runs to the next LF; no LF before EOF means the closing
            // boundary is missing.
            let Some(lf) = find_lf(input, cursor) else {
                return Err(Error::UnterminatedMetadata);
            };
            let line = &input[cursor..lf];
            cursor = lf + 1;
            parse_metadata_line(line, &mut metadata, &mut pending_comments)?;
        }

        // After the closing boundary: a blank line means a body follows; EOF means
        // no body; anything else is the missing-separator violation.
        let body = match input.get(cursor) {
            None => None,
            Some(b'\n') => Some(input[cursor + 1..].to_vec()),
            Some(_) => return Err(Error::MissingSeparator),
        };

        if body.is_some() && metadata.iter().any(|l| l.prefix == Prefix::Blob) {
            return Err(Error::InlineBodyWithAtReference);
        }

        Ok(Document {
            metadata,
            trailing_comments: pending_comments,
            body,
        })
    }
}

/// True when `input[pos..]` begins with the exact boundary `---\n`. Any deviation
/// — trailing space, a CR before the LF, a fourth hyphen — is not a boundary.
fn boundary_at(input: &[u8], pos: usize) -> bool {
    input.len() >= pos + BOUNDARY.len() && &input[pos..pos + BOUNDARY.len()] == BOUNDARY
}

/// The index of the next LF at or after `pos`, or `None` if there is none.
fn find_lf(input: &[u8], pos: usize) -> Option<usize> {
    input[pos..]
        .iter()
        .position(|&b| b == b'\n')
        .map(|i| pos + i)
}

/// Parse one metadata line (LF already stripped) into either a pending comment or
/// a `MetadataLine` that adopts the accumulated comments.
fn parse_metadata_line(
    line: &[u8],
    metadata: &mut Vec<MetadataLine>,
    pending_comments: &mut Vec<String>,
) -> Result<(), Error> {
    // Prefix validity is checked before the space rule, so `X bad` is an invalid
    // prefix while `!nospace` is a malformed line.
    let prefix = line
        .first()
        .copied()
        .and_then(Prefix::from_byte)
        .ok_or(Error::InvalidPrefix)?;

    if line.get(1) != Some(&b' ') {
        return Err(Error::MalformedLine);
    }

    // Content is UTF-8 per the grammar; reject bytes that are not.
    let value = std::str::from_utf8(&line[2..])
        .map_err(|_| Error::MalformedLine)?
        .to_owned();

    if prefix == Prefix::Comment {
        pending_comments.push(value);
    } else {
        metadata.push(MetadataLine {
            prefix,
            value,
            leading_comments: std::mem::take(pending_comments),
        });
    }
    Ok(())
}
