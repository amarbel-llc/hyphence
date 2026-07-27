//go:build test

package hyphence

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// vectorsExcludedFromGrammarCheck are RFC 0002 vectors that deliberately
// exercise the RETIRED, pre-RFC-0003 spaced-lock spelling (`Lock = SP
// DigestTerm`) to prove it still decodes at the envelope layer — RFC
// 0002's own vector-comment header says these "do not exercise RFC
// 0002's content-grammar productions themselves." hyphence-content.peg
// formalizes the CURRENT (RFC 0003 MarklTerm) grammar, so feeding these
// retired-spelling vectors through it would test the wrong shape.
var vectorsExcludedFromGrammarCheck = map[string]bool{
	"unified-lock-type":               true,
	"unified-lock-reference":          true,
	"deprecated-angle-still-accepted": true,
}

// grammarCheckableOutcomes are the rfc_vectors.txt outcomes whose
// metadata section is known to decode without error, so its
// MetadataLine.Value entries are safe to extract and cross-check.
// document/parse-error-inline-body-with-at is included even though its
// overall document-level outcome is an error: per its own outcome doc
// comment in testdata/rfc_vectors.txt, "Reader must succeed" for the
// metadata section — the error is a later, semantic (inline-body) check,
// not a metadata-decode failure.
var grammarCheckableOutcomes = map[string]bool{
	"legacy/parse-ok":                          true,
	"document/parse-ok":                        true,
	"document/parse-error-inline-body-with-at": true,
	// grammar/reject is this harness's OWN namespace (hyphence#11): the
	// envelope decodes fine, but the content must NOT parse under any
	// real content production. Listed here so the vector reaches the
	// per-line loop below, where the assertion is inverted.
	grammarRejectOutcome: true,
}

// grammarRejectOutcome marks a vector whose content-grammar-governed
// metadata lines MUST NOT parse under hyphence-content.peg via a real
// content production. It is the only NEGATIVE outcome this file owns,
// and it exists because the corpus had no way to express one: every
// other outcome asserts a successful parse, so a malformed-content
// vector added under any of them would fail the FreeText-fallback check
// in assertParsesUnderGrammar rather than pinning the rejection.
//
// The envelope harnesses (rfc_conformance_test.go, document_test.go)
// skip this namespace per the vector file's own namespacing rule, so a
// grammar/reject vector's document still has to be envelope-VALID —
// only its content is ill-formed.
const grammarRejectOutcome = "grammar/reject"

// langlangStartRule is hyphence-content.peg's first (start) rule.
// langlang's `-input` mode always parses against the grammar's start
// rule (there is no rule-selection flag) — a successful parse's stdout
// is a highlighted tree dump rooted at that name; a failed parse
// instead prints a `path:line:col: message` line and (as of this
// writing) langlang exits 0 either way, so the tree-dump prefix is the
// only reliable success signal.
const langlangStartRule = "HyphenceContent"

var langlangFailurePattern = regexp.MustCompile(`^\S+:\d+:\d+: `)

// langlang's tree-dump output is ANSI-colored by default (no -no-color
// flag exists as of this writing) — strip SGR escape sequences before
// checking for langlangStartRule, or a literal-prefix check would
// always see the escape sequence first and never match.
var ansiSGRPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// TestGrammarVectors cross-checks each RFC test vector's content-
// grammar-governed metadata lines (!, @, -, <) against
// docs/rfcs/hyphence-content.peg via langlang -input, closing the gap
// flagged in hyphence#9: validate-grammar previously only checked that
// the .peg is well-formed, never that real hyphence content parses
// under it. Requires the `langlang` binary and the .peg file's path;
// skips (not fails) when either isn't available, so a plain `go test
// -tags test ./...` outside the Nix-wired gate still passes — the
// enforced check is `just test-grammar-vectors` / `nix build
// .#grammar-vectors-test`, which sets both.
func TestGrammarVectors(t *testing.T) {
	langlangBin, err := resolveLanglangBin()
	if err != nil {
		t.Skipf("skipping grammar-vector cross-check: %v (see hyphence#9)", err)
	}

	grammarPeg, err := resolveGrammarPeg()
	if err != nil {
		t.Skipf("skipping grammar-vector cross-check: %v (see hyphence#9)", err)
	}

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

			// A grammar/reject vector asserts the DOCUMENT carries
			// content the grammar refuses — at least one
			// content-governed line must fail to match a real
			// production. "At least one" rather than "all": a realistic
			// negative can pair a well-formed line with an ill-formed
			// one (`- good=value` beside `- bad=@junk`), and demanding
			// every line be rejected would forbid that shape.
			expectReject := v.outcome == grammarRejectOutcome
			sawReject := false

			for _, ml := range doc.Metadata {
				switch ml.Prefix {
				case '!', '@', '-', '<':
				default:
					continue
				}

				if expectReject {
					if ok, _ := parsesUnderGrammar(t, langlangBin, grammarPeg, ml.Value); !ok {
						sawReject = true
					}

					continue
				}

				t.Run(string(ml.Prefix), func(t *testing.T) {
					assertParsesUnderGrammar(t, langlangBin, grammarPeg, ml.Value)
				})
			}

			if expectReject && !sawReject {
				t.Errorf("outcome %s: every content-governed line parsed under hyphence-content.peg, but this vector asserts at least one must be rejected — the grammar may have regressed to a permissive production", grammarRejectOutcome)
			}
		})
	})
}

func resolveLanglangBin() (string, error) {
	if bin := os.Getenv("LANGLANG_BIN"); bin != "" {
		return bin, nil
	}
	return exec.LookPath("langlang")
}

func resolveGrammarPeg() (string, error) {
	if p := os.Getenv("HYPHENCE_GRAMMAR_PEG"); p != "" {
		return p, nil
	}
	p := filepath.Join("..", "..", "docs", "rfcs", "hyphence-content.peg")
	if _, err := os.Stat(p); err != nil {
		return "", err
	}
	return p, nil
}

// assertParsesUnderGrammar fails the test unless content matches a real
// content-grammar production.
func assertParsesUnderGrammar(t *testing.T, langlangBin, grammarPeg, content string) {
	t.Helper()

	if ok, detail := parsesUnderGrammar(t, langlangBin, grammarPeg, content); !ok {
		t.Errorf("content %q did not parse under hyphence-content.peg via a real content production:\n%s", content, detail)
	}
}

// parsesUnderGrammar runs one content string through langlang against
// hyphence-content.peg and reports whether it matched a REAL content
// production — DashContent/TypeContent/BlobContent and their
// descendants — as opposed to failing outright or falling through to the
// FreeText catch-all. detail carries langlang's output for diagnostics.
//
// Splitting this predicate out of the assertion is what lets the
// grammar/reject outcome reuse the identical parse logic with an
// inverted expectation, instead of a second near-copy that could drift
// from this one.
func parsesUnderGrammar(t *testing.T, langlangBin, grammarPeg, content string) (bool, string) {
	t.Helper()

	tmp, err := os.CreateTemp(t.TempDir(), "hyphence-grammar-vector-*")
	if err != nil {
		t.Fatalf("create temp input: %v", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("write temp input: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp input: %v", err)
	}

	// NOT -grammar-ast: that flag returns early after printing the
	// compiled grammar's AST, before any -input matching happens.
	cmd := exec.Command(
		langlangBin,
		"-grammar", grammarPeg,
		"-input", tmp.Name(),
		"-disable-builtins",
		"-disable-spaces",
	)
	// Separate stdout/stderr rather than CombinedOutput(): interleaving
	// order between the two streams is unspecified, and the tree-dump
	// vs. failure-line checks below both key off stdout starting with a
	// specific token — any concurrently-written stderr text (a warning,
	// say) could land first in a combined buffer and break that check
	// spuriously. stderr is kept only for the error message.
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmdErr := cmd.Run()
	trimmed := strings.TrimSpace(ansiSGRPattern.ReplaceAllString(stdout.String(), ""))
	detail := "stdout: " + stdout.String() + "\nstderr: " + stderr.String()

	if cmdErr != nil || !strings.HasPrefix(trimmed, langlangStartRule) || langlangFailurePattern.MatchString(trimmed) {
		return false, detail
	}

	// HyphenceContent's last alternative, FreeText, matches ANY
	// single-line string unconditionally (RFC 0001 line-splitting
	// guarantees MetadataLine.Value never contains a literal LF, and
	// FreeText is `(!LF .)*`) — so a value that fails every real
	// content-grammar production (DashContent/TypeContent/BlobContent)
	// still makes the overall parse succeed via that catch-all, and the
	// tree dump still starts with "HyphenceContent" regardless. None of
	// DashContent/TypeContent/BlobContent's own productions ever
	// reference FreeText, so its name appearing anywhere in a
	// successful tree only happens via that fallthrough — without this
	// check, this test would have no power to catch a real content-
	// grammar regression (hyphence#9's own reason for existing). If a
	// FUTURE vector legitimately needs the old, retired notation (like
	// the three already in vectorsExcludedFromGrammarCheck), add it
	// there rather than letting it fail here — or, if the point IS that
	// the content must be refused, give it the grammar/reject outcome.
	if strings.Contains(trimmed, "FreeText") {
		return false, "content matched only HyphenceContent's FreeText fallback, not a real content-grammar production — if this is deliberately a retired-spelling vector, add it to vectorsExcludedFromGrammarCheck; if it should be refused outright, give it the " + grammarRejectOutcome + " outcome:\n" + detail
	}

	return true, detail
}
