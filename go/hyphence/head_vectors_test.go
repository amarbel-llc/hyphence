//go:build test

package hyphence

import (
	"bytes"
	"encoding/base64"
	"testing"
)

// headCanonicalOutcome marks a vector whose input must decode into a typed
// head and encode, under the RFC 0005 canonical encoder, to exactly the
// expected field's bytes.
const headCanonicalOutcome = "head/canonical"

// TestHeadCanonicalVectors runs every head/canonical vector through the
// typed head: decode, encode canonically, compare with the expected bytes,
// then decode and re-encode those bytes to prove the canonical form is a
// fixed point. Rust's conformance.rs head_canonical_vectors runs the
// identical check, which is what makes the two encoders agree byte for
// byte.
func TestHeadCanonicalVectors(t *testing.T) {
	checked := 0

	forEachRFCVector(t, func(v rfcVector) {
		if v.outcome != headCanonicalOutcome {
			return
		}

		checked++

		t.Run(v.name, func(t *testing.T) {
			expected, err := base64.StdEncoding.DecodeString(v.expectedB64)
			if err != nil {
				t.Fatalf("decode expected b64: %v", err)
			}

			for _, tc := range []struct {
				label string
				input []byte
			}{
				{"input", v.input},
				{"expected", expected},
			} {
				head, body, hasBody, err := DecodeHead(bytes.NewReader(tc.input))
				if err != nil {
					t.Fatalf("%s did not decode as a typed head: %v", tc.label, err)
				}

				var got bytes.Buffer
				if err := head.Encode(&got, body, hasBody); err != nil {
					t.Fatalf("%s did not encode: %v", tc.label, err)
				}

				if !bytes.Equal(got.Bytes(), expected) {
					t.Errorf("canonical encoding of the %s differs:\n got: %q\nwant: %q", tc.label, got.String(), string(expected))
				}
			}
		})
	})

	if checked == 0 {
		t.Errorf("no %s vectors were checked", headCanonicalOutcome)
	}
}
