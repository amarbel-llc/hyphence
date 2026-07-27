//! RFC 0001 conformance against the normative test vectors.
//!
//! `testdata/rfc_vectors.txt` is the shared canonical vector file (copied verbatim
//! from madder's Go reference implementation). Every conforming implementation MUST
//! agree with these outcomes; this harness runs all of them against `decode`.

use crate::{Document, Error, parse_content};

/// Vectors that deliberately exercise the RETIRED, pre-RFC-0003 spaced-lock
/// spelling (`Lock = SP DigestTerm`) to prove it still decodes at the envelope
/// layer. They are envelope-valid but do NOT parse under the current content
/// grammar, so the content cross-check below skips them — mirroring
/// `vectorsExcludedFromGrammarCheck` in the Go harness.
const RETIRED_SPELLING_VECTORS: [&str; 3] = [
    "unified-lock-type",
    "unified-lock-reference",
    "deprecated-angle-still-accepted",
];

/// Decode standard base64 (RFC 4648 alphabet), ignoring `=` padding and ASCII
/// whitespace. Zero-dependency; only needs to handle the vector file's payloads.
fn b64_decode(s: &str) -> Vec<u8> {
    fn sextet(c: u8) -> Option<u8> {
        Some(match c {
            b'A'..=b'Z' => c - b'A',
            b'a'..=b'z' => c - b'a' + 26,
            b'0'..=b'9' => c - b'0' + 52,
            b'+' => 62,
            b'/' => 63,
            _ => return None,
        })
    }
    let mut out = Vec::new();
    let mut acc: u32 = 0;
    let mut bits = 0u32;
    for c in s.bytes() {
        if c == b'=' || c.is_ascii_whitespace() {
            continue;
        }
        let v = sextet(c).expect("vector payload is valid base64") as u32;
        acc = (acc << 6) | v;
        bits += 6;
        if bits >= 8 {
            bits -= 8;
            out.push((acc >> bits) as u8);
        }
    }
    out
}

#[test]
fn rfc_test_vectors() {
    let vectors = include_str!("../testdata/rfc_vectors.txt");
    let mut ran = 0;

    for (i, raw) in vectors.lines().enumerate() {
        let line = raw.trim_end_matches('\r');
        if line.is_empty() || line.starts_with('#') {
            continue;
        }

        let parts: Vec<&str> = line.split('\t').collect();
        assert_eq!(
            parts.len(),
            4,
            "line {}: want 4 tab-separated fields, got {}",
            i + 1,
            parts.len()
        );
        let (name, input_b64, outcome, expected_b64) = (parts[0], parts[1], parts[2], parts[3]);

        let result = Document::decode(&b64_decode(input_b64));

        match outcome {
            "legacy/parse-ok" => {
                let doc = result.unwrap_or_else(|e| panic!("{name}: expected parse-ok, got {e:?}"));
                let expected = if expected_b64 == "-" {
                    Vec::new()
                } else {
                    b64_decode(expected_b64)
                };
                assert_eq!(
                    doc.body.unwrap_or_default(),
                    expected,
                    "{name}: body mismatch"
                );
            }
            "legacy/parse-error-missing-separator" => {
                assert_eq!(result, Err(Error::MissingSeparator), "{name}");
            }
            "document/parse-ok" => {
                result.unwrap_or_else(|e| panic!("{name}: expected parse-ok, got {e:?}"));
            }
            "document/parse-error-invalid-prefix" => {
                assert_eq!(result, Err(Error::InvalidPrefix), "{name}");
            }
            "document/parse-error-malformed-line" => {
                assert_eq!(result, Err(Error::MalformedLine), "{name}");
            }
            // Per this outcome's own definition in the vector file, decoding
            // MUST SUCCEED and the cross-line state must then show the
            // violation. Before hyphence#11 this arm asserted decode returned
            // Err, which contradicted the corpus prose and diverged from Go
            // (whose Reader succeeds and whose validate subcommand does the
            // cross-check). The rule moved out of Document::decode; the
            // assertion follows it.
            "document/parse-error-inline-body-with-at" => {
                let doc = result.unwrap_or_else(|e| {
                    panic!("{name}: decode must succeed for this outcome, got {e:?}")
                });
                assert!(
                    doc.has_inline_body_with_blob_ref(),
                    "{name}: expected the @-line-plus-body violation to be detectable"
                );
            }
            // Outcomes are namespaced by owning harness so several
            // harnesses can share this one canonical vector file, and the
            // file's own header documents that a foreign namespace is
            // SKIPPED. This harness owns `legacy/` and `document/` — the
            // envelope outcomes. `grammar/` belongs to the Go
            // content-grammar harness (grammar_vectors_test.go), which
            // needs langlang and the .peg, neither of which this crate
            // has; its vectors are still envelope-valid, they just are
            // not this harness's to assert.
            //
            // Before hyphence#11 this arm panicked on ANY unrecognized
            // outcome, which contradicted both the documented rule and
            // both Go harnesses (each of which skips outside its own
            // prefix). Adding the `grammar/` namespace is what surfaced
            // the divergence.
            other => {
                assert!(
                    !other.starts_with("legacy/") && !other.starts_with("document/"),
                    "{name}: unknown outcome {other:?} inside this harness's own namespace"
                );
                continue;
            }
        }
        ran += 1;
    }

    // A LOWER bound, not an equality: the previous `== 19` had to be
    // hand-bumped for every vector added, and adding one is otherwise a
    // testdata-only edit (the file header promises exactly that). A floor
    // still catches the regression that assertion existed for — vectors
    // silently ceasing to be exercised — without turning every new vector
    // into a source change here.
    assert!(
        ran >= 19,
        "expected to exercise at least the nineteen envelope RFC vectors, ran {ran}"
    );
}

/// Runs the shared vector corpus through [`parse_content`] and asserts it reaches
/// the same verdict the normative grammar does.
///
/// This is the lockstep gate for the Rust content parser. `rfc_test_vectors`
/// above proves the ENVELOPE agrees across impls; the unit tables in `content.rs`
/// prove hand-picked shapes. Neither would catch this parser and
/// `docs/rfcs/hyphence-content.peg` disagreeing about a real vector — which is
/// the drift that matters, since the `.peg` is normative and this parser is
/// supposed to implement it. The Go side runs the identical check
/// (`TestContentParserAgreesWithVectorCorpus`) plus a langlang cross-check
/// against the `.peg` itself, which this crate cannot do (no langlang, no `.peg`
/// path). Sharing one corpus is what transitively ties this parser to the
/// grammar.
#[test]
fn content_grammar_vectors() {
    let vectors = include_str!("../testdata/rfc_vectors.txt");
    let mut checked = 0;

    for raw in vectors.lines() {
        let line = raw.trim_end_matches('\r');
        if line.is_empty() || line.starts_with('#') {
            continue;
        }

        let parts: Vec<&str> = line.split('\t').collect();
        let (name, input_b64, outcome) = (parts[0], parts[1], parts[2]);

        if RETIRED_SPELLING_VECTORS.contains(&name) {
            continue;
        }

        // Only outcomes whose document DECODES can be content-checked, since the
        // metadata lines have to be in hand first.
        //
        // `document/parse-error-inline-body-with-at` qualifies as of hyphence#11:
        // its violation is a cross-line rule the decoder no longer enforces, so
        // decoding succeeds and its lines are reachable. That closes the gap
        // where this harness checked one fewer vector than the Go one for a
        // reason that was really a Rust/Go divergence rather than a property of
        // the vector.
        let expect_reject = match outcome {
            "legacy/parse-ok"
            | "document/parse-ok"
            | "document/parse-error-inline-body-with-at" => false,
            "grammar/reject" => true,
            _ => continue,
        };

        let doc = Document::decode(&b64_decode(input_b64))
            .unwrap_or_else(|e| panic!("{name}: expected the envelope to decode, got {e:?}"));

        let mut saw_reject = false;

        for line in &doc.metadata {
            let result = parse_content(line.prefix, &line.value);

            if expect_reject {
                saw_reject |= result.is_err();
                continue;
            }

            assert_eq!(
                result,
                Ok(()),
                "{name}: parse_content rejected {:?}, but the vector conforms to \
                 hyphence-content.peg — this parser disagrees with the normative grammar",
                line.value
            );
        }

        // "At least one" rather than "all": a negative vector may legitimately
        // pair a well-formed line with an ill-formed one. Same semantics the Go
        // harness uses for this outcome.
        assert!(
            !expect_reject || saw_reject,
            "{name}: parse_content accepted every line, but the grammar/reject \
             outcome asserts at least one must be refused"
        );

        checked += 1;
    }

    // Guards against the gating above silently excluding everything, which would
    // make this test vacuously pass.
    assert!(
        checked > 0,
        "no vectors were content-checked; the outcome gating has drifted from the corpus"
    );
}
