//! RFC 0001 conformance against the normative test vectors.
//!
//! `testdata/rfc_vectors.txt` is the shared canonical vector file (copied verbatim
//! from madder's Go reference implementation). Every conforming implementation MUST
//! agree with these outcomes; this harness runs all of them against `decode`.

use crate::{Document, Error};

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
            "document/parse-error-inline-body-with-at" => {
                assert_eq!(result, Err(Error::InlineBodyWithAtReference), "{name}");
            }
            other => panic!("{name}: unknown outcome {other:?}"),
        }
        ran += 1;
    }

    assert_eq!(
        ran, 12,
        "expected to exercise all twelve RFC vectors, ran {ran}"
    );
}
