//! hyphence: a zero-dependency Rust implementation of the hyphence serialization
//! format (RFC 0001).
//!
//! A hyphence document is a `---`-fenced **metadata section** followed by an
//! optional **opaque body**:
//!
//! ```text
//! ---
//! # a description
//! ! inventory_list-v2
//! ---
//!
//! <arbitrary body bytes>
//! ```
//!
//! The metadata lines are keyed by a single-character prefix (`!` type, `@` blob
//! reference, `#` description, `-` tag/reference, `<` object reference, `%`
//! comment); the body's interpretation is the concern of the `!` type line and is
//! opaque here. The format itself is **unversioned** — evolution is carried by the
//! type string's `-vN` suffix, and decoders retain support for old type strings
//! indefinitely. See RFC 0001 for the normative wire format.
//!
//! This crate implements the document **envelope** — [`Document::decode`] and
//! [`Document::encode`] — not the type-aware body coders. It is dependency-free and
//! self-contained; it is the Rust sibling of the Go implementation under
//! `go/hyphence/` in this repo, and both are checked against the same RFC 0001
//! conformance vectors.

mod decode;
mod document;
mod encode;
mod error;

pub use document::{Document, MetadataLine, Prefix};
pub use error::Error;

#[cfg(test)]
mod conformance;
#[cfg(test)]
mod tests;
