//! RFC 0001 encoder: `Document` -> canonical bytes.

use crate::document::{Document, MetadataLine};

impl Document {
    /// Serialize to canonical hyphence bytes.
    ///
    /// Metadata lines are emitted in the canonical order of RFC 0001
    /// §Encoder Behavior (descriptions, references/tags, the `@` blob line, the `!`
    /// type line), each `%` comment emitted just before the line it is entangled
    /// with. Boundaries are exactly `---\n`; the single blank separator line is
    /// emitted only when a body is present (a body-less document ends right after
    /// the closing boundary). Round-trips: `decode(encode(d))` recovers `d`.
    pub fn encode(&self) -> Vec<u8> {
        let mut out = Vec::new();
        let has_metadata = !self.metadata.is_empty() || !self.trailing_comments.is_empty();

        if has_metadata {
            out.extend_from_slice(b"---\n");

            // Stable sort into canonical order, keeping source order within a rank.
            let mut ordered: Vec<&MetadataLine> = self.metadata.iter().collect();
            ordered.sort_by_key(|l| l.prefix.order_rank());

            for line in ordered {
                for comment in &line.leading_comments {
                    emit_line(&mut out, b'%', comment);
                }
                emit_line(&mut out, line.prefix.byte(), &line.value);
            }
            for comment in &self.trailing_comments {
                emit_line(&mut out, b'%', comment);
            }

            out.extend_from_slice(b"---\n");

            if let Some(body) = &self.body {
                out.push(b'\n'); // the required blank separator line
                out.extend_from_slice(body);
            }
        } else if let Some(body) = &self.body {
            // No metadata section at all: the document is just its body bytes.
            out.extend_from_slice(body);
        }

        out
    }
}

/// Emit one metadata line: `<prefix> <content>\n`.
fn emit_line(out: &mut Vec<u8>, prefix: u8, content: &str) {
    out.push(prefix);
    out.push(b' ');
    out.extend_from_slice(content.as_bytes());
    out.push(b'\n');
}
