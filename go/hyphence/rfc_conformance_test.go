//go:build test

package hyphence

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// Test vectors live in testdata/rfc_vectors.txt; format is documented in
// docs/rfcs/0001-hyphence.md (and the file's own comment header). Every
// conforming implementation MUST agree with these outcomes. Adding a new
// vector requires only a new line in the testdata file — no test-code
// edits.
func TestRFCConformance_HyphenceTestVectors(t *testing.T) {
	forEachRFCVector(t, func(v rfcVector) {
		t.Run(v.name, func(t *testing.T) {
			runRFCVector(t, v.input, v.outcome, v.expectedB64)
		})
	})
}

func runRFCVector(t *testing.T, input []byte, outcome, expectedB64 string) {
	t.Helper()

	var blobCapture bytes.Buffer

	decoder := Decoder[*TypedBlobDefault[struct{}]]{
		Metadata:      TypedMetadataCoderDefault[struct{}]{},
		Blob:          testBlobDecoder{},
		BlobTeeWriter: &blobCapture,
	}

	typedBlob := &TypedBlobDefault[struct{}]{}
	reader := bufio.NewReader(bytes.NewReader(input))

	_, err := decoder.DecodeFrom(typedBlob, reader)

	switch outcome {
	case "legacy/parse-ok":
		if err != nil {
			t.Fatalf("expected parse-ok, got error: %v", err)
		}

		var expected []byte
		if expectedB64 != "" && expectedB64 != "-" {
			expected, err = base64.StdEncoding.DecodeString(expectedB64)
			if err != nil {
				t.Fatalf("decode expected b64: %v", err)
			}
		}

		if !bytes.Equal(blobCapture.Bytes(), expected) {
			t.Errorf("blob mismatch: got %q, want %q", blobCapture.String(), string(expected))
		}

	case "legacy/parse-error-missing-separator":
		if err == nil {
			t.Fatal("expected parse-error-missing-separator, got nil")
		}
		if !errors.Is(err, errMissingNewlineAfterBoundary) {
			t.Errorf("expected errMissingNewlineAfterBoundary, got: %v", err)
		}

	default:
		if !strings.HasPrefix(outcome, "legacy/") {
			t.Skipf("outcome %q owned by another harness", outcome)
		}
		t.Fatalf("unknown outcome %q in legacy/ namespace", outcome)
	}
}
