//! Round-trip, accessor, and boundary-strictness units (complementary to the RFC
//! conformance vectors in `conformance.rs`).

use crate::Document;

/// hyphence#11: the `@`-line-plus-body rule is a CROSS-LINE check the decoder
/// leaves to the caller, so a document carrying both DECODES successfully and the
/// caller detects it via `has_inline_body_with_blob_ref`.
///
/// This is what the shared corpus's `document/parse-error-inline-body-with-at`
/// outcome actually specifies ("Reader must succeed; the post-ReadFrom validator
/// state must show..."), and what Go does. `decode` returning `Err` here — the
/// prior behavior — diverged from both.
#[test]
fn inline_body_with_blob_ref_decodes_and_is_detectable() {
    let doc = Document::decode(b"---\n@ blake2b256-abc\n! md\n---\n\ninline\n")
        .expect("decode must succeed: the rule is caller-enforced, not decoder-enforced");

    assert!(doc.has_inline_body_with_blob_ref());
    assert_eq!(doc.body.as_deref(), Some(&b"inline\n"[..]));
    // The lines stay reachable, which is what lets the content-grammar
    // cross-check cover this vector at all.
    assert_eq!(doc.blob_ref(), Some("blake2b256-abc"));
    assert_eq!(doc.type_line(), Some("md"));

    // An `@` line with no body is not a violation.
    let no_body = Document::decode(b"---\n@ blake2b256-abc\n! md\n---\n").unwrap();
    assert!(!no_body.has_inline_body_with_blob_ref());

    // A body with no `@` line is not a violation.
    let no_blob_ref = Document::decode(b"---\n! md\n---\n\ninline\n").unwrap();
    assert!(!no_blob_ref.has_inline_body_with_blob_ref());
}

#[test]
fn type_only_round_trips_byte_exact() {
    let bytes = b"---\n! md\n---\n";
    let doc = Document::decode(bytes).unwrap();
    assert_eq!(doc.type_line(), Some("md"));
    assert_eq!(doc.body, None);
    assert_eq!(doc.encode(), bytes);
}

#[test]
fn type_and_body_round_trips_with_binary_body() {
    let doc = Document::with_type_and_body(
        "mephisto-population-controller-schema_v1",
        vec![0u8, 1, 2, 0xff, b'\n', b'-', b'-', b'-'],
    );
    let bytes = doc.encode();
    let back = Document::decode(&bytes).unwrap();
    assert_eq!(back, doc, "decode(encode(d)) must recover d");
    assert_eq!(back.encode(), bytes, "encode must be byte-stable");
    assert_eq!(
        back.type_line(),
        Some("mephisto-population-controller-schema_v1")
    );
}

#[test]
fn descriptions_and_entangled_comment_round_trip() {
    // The `%` comment is entangled with the `! task` line that follows it, so a
    // canonical re-emit keeps it attached and the bytes are stable.
    let bytes = b"---\n# first\n# second\n% a comment\n! task\n---\n";
    let doc = Document::decode(bytes).unwrap();
    let descriptions: Vec<&str> = doc.descriptions().collect();
    assert_eq!(descriptions, ["first", "second"]);
    assert_eq!(doc.encode(), bytes);
}

#[test]
fn canonical_order_reorders_to_spec() {
    // Source order is type-then-description; the encoder emits descriptions before
    // the type line per RFC 0001 canonical order.
    let source = b"---\n! task\n# desc\n---\n";
    let doc = Document::decode(source).unwrap();
    assert_eq!(doc.encode(), b"---\n# desc\n! task\n---\n");
}

#[test]
fn malformed_boundaries_are_not_boundaries() {
    // A trailing space, a CR, or a fourth hyphen each disqualifies the line as a
    // boundary; with no real opening boundary the whole input is the body.
    for bad in [
        &b"--- \n! md\n--- \n"[..],
        &b"---\r\n! md\n---\r\n"[..],
        &b"----\n! md\n----\n"[..],
    ] {
        let doc = Document::decode(bad).unwrap();
        assert!(
            doc.metadata.is_empty(),
            "{bad:?} must not parse a metadata section"
        );
        assert_eq!(doc.body.as_deref(), Some(bad));
    }
}

#[test]
fn no_opening_boundary_is_all_body() {
    let bytes = b"just some bytes\nwith no fence\n";
    let doc = Document::decode(bytes).unwrap();
    assert!(doc.metadata.is_empty());
    assert_eq!(doc.body.as_deref(), Some(&bytes[..]));
}

#[test]
fn empty_input_is_an_empty_document() {
    let doc = Document::decode(b"").unwrap();
    assert!(doc.metadata.is_empty());
    assert_eq!(doc.body, None);
    assert_eq!(doc.encode(), b"");
}
