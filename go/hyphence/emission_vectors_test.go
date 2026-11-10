//go:build test

package hyphence

import (
	"bytes"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
)

// emissionVector is one record of testdata/0005-vectors.txt; the format is
// in that file's header.
type emissionVector struct {
	name         string
	outcome      string
	rule         string
	document     []byte
	canonical    []byte
	hasCanonical bool
}

func readEmissionVectors(t *testing.T) []emissionVector {
	t.Helper()

	bites, err := os.ReadFile("testdata/0005-vectors.txt")
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}

	const (
		blockNone = iota
		blockDocument
		blockCanonical
	)

	var (
		vectors []emissionVector
		current *emissionVector
		block   = blockNone
		body    []byte
	)

	finish := func() {
		if current == nil {
			return
		}
		current.document = append(current.document, body...)
		if current.hasCanonical {
			current.canonical = append(current.canonical, body...)
		}
		vectors = append(vectors, *current)
		current, body = nil, nil
	}

	for _, line := range strings.Split(strings.TrimSuffix(string(bites), "\n"), "\n") {
		if line == "" {
			finish()
			block = blockNone
			continue
		}

		if current == nil {
			if strings.HasPrefix(line, "#") {
				continue
			}
			name, ok := strings.CutPrefix(line, "name: ")
			if !ok {
				t.Fatalf("record does not start with name: %q", line)
			}
			current = &emissionVector{name: name}
			continue
		}

		if text, ok := strings.CutPrefix(line, "| "); ok || line == "|" {
			if !ok {
				text = ""
			}

			switch block {
			case blockDocument:
				current.document = append(current.document, text+"\n"...)
			case blockCanonical:
				current.canonical = append(current.canonical, text+"\n"...)
			default:
				t.Fatalf("%s: a | line outside a document block", current.name)
			}
			continue
		}

		block = blockNone
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			t.Fatalf("%s: not a key: value line: %q", current.name, line)
		}
		value = strings.TrimLeft(value, " ")

		switch key {
		case "outcome":
			current.outcome = value
		case "rule":
			current.rule = value
		case "body-hex":
			if body, err = hex.DecodeString(value); err != nil {
				t.Fatalf("%s: body-hex: %v", current.name, err)
			}
		case "document":
			block = blockDocument
		case "canonical":
			current.hasCanonical = true
			block = blockCanonical
		default:
			t.Fatalf("%s: unknown key %q", current.name, key)
		}
	}
	finish()

	return vectors
}

// TestEmissionVectors replays testdata/0005-vectors.txt, the RFC 0005
// vectors piggy also replays. A canonical record's document and canonical
// form both encode to the canonical form; a refuse record fails to decode
// for the named rule. Rust's conformance.rs emission_vectors runs the
// identical check.
func TestEmissionVectors(t *testing.T) {
	vectors := readEmissionVectors(t)
	if len(vectors) == 0 {
		t.Fatal("no RFC 0005 vectors were parsed")
	}

	for _, v := range vectors {
		t.Run(v.name, func(t *testing.T) {
			switch {
			case v.outcome == "canonical" && v.hasCanonical && v.rule == "":
				for _, tc := range []struct {
					label string
					input []byte
				}{
					{"document", v.document},
					{"canonical", v.canonical},
				} {
					head, body, hasBody, err := DecodeHead(bytes.NewReader(tc.input))
					if err != nil {
						t.Fatalf("%s did not decode: %v", tc.label, err)
					}

					var got bytes.Buffer
					if err := head.Encode(&got, body, hasBody); err != nil {
						t.Fatalf("%s did not encode: %v", tc.label, err)
					}

					if !bytes.Equal(got.Bytes(), v.canonical) {
						t.Errorf("canonical encoding of the %s differs:\n got: %q\nwant: %q", tc.label, got.String(), string(v.canonical))
					}
				}

			case v.outcome == "refuse" && !v.hasCanonical && v.rule != "":
				_, _, _, err := DecodeHead(bytes.NewReader(v.document))
				if err == nil {
					t.Fatalf("decoded, want a refusal under rule %s", v.rule)
				}

				got := "envelope"
				if errors.Is(err, ErrMalformedContent) {
					got = "content"
				}
				if got != v.rule {
					t.Errorf("refused under rule %s, want %s: %v", got, v.rule, err)
				}

			default:
				t.Fatal("malformed record (outcome, canonical and rule disagree)")
			}
		})
	}
}
