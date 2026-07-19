//go:build test

package hyphence

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// rfcVector is one parsed, not-yet-consumed row from
// testdata/rfc_vectors.txt (name, decoded input, outcome, and the raw
// expected-b64 field — decoding the latter is outcome-specific, so
// callers do it themselves).
type rfcVector struct {
	name        string
	input       []byte
	outcome     string
	expectedB64 string
}

// forEachRFCVector reads testdata/rfc_vectors.txt and calls fn once per
// non-comment, non-blank vector line, after base64-decoding its input
// field. Shared by every harness that scans this file (rfc_conformance_test.go,
// grammar_vectors_test.go) so the tab-separated-format parsing lives in
// exactly one place. Malformed lines (wrong field count, bad base64)
// are reported via t.Errorf and skipped, not fatal, so one bad line
// doesn't hide failures in the rest of the corpus.
func forEachRFCVector(t *testing.T, fn func(v rfcVector)) {
	t.Helper()

	bites, err := os.ReadFile("testdata/rfc_vectors.txt")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}

	for lineNo, raw := range strings.Split(string(bites), "\n") {
		line := strings.TrimRight(raw, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 4 {
			t.Errorf("line %d: want 4 tab-separated fields, got %d", lineNo+1, len(parts))
			continue
		}

		name, inputB64, outcome, expectedB64 := parts[0], parts[1], parts[2], parts[3]

		input, err := base64.StdEncoding.DecodeString(inputB64)
		if err != nil {
			t.Errorf("%s: decode input b64: %v", name, err)
			continue
		}

		fn(rfcVector{name: name, input: input, outcome: outcome, expectedB64: expectedB64})
	}
}
