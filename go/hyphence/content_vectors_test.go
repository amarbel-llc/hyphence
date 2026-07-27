//go:build test

package hyphence

import (
	"bytes"
	"testing"
)

// TestContentParserAgreesWithVectorCorpus runs the shared RFC vector
// corpus through ParseContent and asserts it reaches the same verdict the
// langlang cross-check (TestGrammarVectors) reaches against
// docs/rfcs/hyphence-content.peg.
//
// This is the lockstep gate for the Go parser specifically. TestGrammarVectors
// proves the VECTORS conform to the .peg; the unit table in content_test.go
// proves the parser handles hand-picked shapes. Neither would catch the
// parser and the .peg disagreeing about a real vector — which is the drift
// that actually matters, since the .peg is normative and this parser is
// supposed to implement it. Running both over one corpus with one gating
// rule closes that gap without needing langlang here (so this stays a
// plain `go test -tags test` case with no external binary).
//
// The gating mirrors TestGrammarVectors exactly, reusing its own maps:
// retired-spelling vectors are excluded, only grammar-checkable outcomes
// participate, and grammar/reject inverts the expectation.
func TestContentParserAgreesWithVectorCorpus(t *testing.T) {
	checked := 0

	forEachRFCVector(t, func(v rfcVector) {
		if vectorsExcludedFromGrammarCheck[v.name] || !grammarCheckableOutcomes[v.outcome] {
			return
		}

		t.Run(v.name, func(t *testing.T) {
			doc := &Document{}
			reader := &Reader{
				RequireMetadata: true,
				Metadata:        &MetadataBuilder{Doc: doc},
				Blob:            &CountingDiscardReaderFrom{},
			}

			if _, err := reader.ReadFrom(bytes.NewReader(v.input)); err != nil {
				t.Fatalf("decode vector: %v", err)
			}

			// Same "at least one line is refused" semantics
			// TestGrammarVectors uses for this outcome: a negative
			// vector may legitimately pair a well-formed line with an
			// ill-formed one.
			expectReject := v.outcome == grammarRejectOutcome
			sawReject := false

			for _, ml := range doc.Metadata {
				switch ml.Prefix {
				case '!', '@', '-', '<':
				default:
					continue
				}

				err := ParseContent(ml.Prefix, ml.Value)

				if expectReject {
					if err != nil {
						sawReject = true
					}

					continue
				}

				if err != nil {
					t.Errorf("ParseContent(%q, %q) = %v, want nil — the vector conforms to hyphence-content.peg, so this parser disagrees with the normative grammar", string(ml.Prefix), ml.Value, err)
				}
			}

			if expectReject && !sawReject {
				t.Errorf("outcome %s: ParseContent accepted every content-governed line, but this vector asserts at least one must be refused", grammarRejectOutcome)
			}
		})

		checked++
	})

	// Guards against the gating maps silently excluding everything (a
	// renamed outcome would otherwise make this test vacuously pass).
	if checked == 0 {
		t.Error("no vectors were checked; the outcome gating above has drifted from the corpus")
	}
}
