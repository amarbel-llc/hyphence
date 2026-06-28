package hyphence

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

// altType / altDigest are a second, non-default instantiation of the type-tag
// and digest constraints, distinct from this package's opaque Type / Digest.
// They prove TypedBlob and the typed coders are genuinely generic over
// T/PT/D/PD — the property madder relies on to instantiate with its native
// ids.TypeStruct / markl.Id rather than the opaque defaults.
type altType struct{ v string }

func (t *altType) Set(s string) error {
	t.v = strings.TrimPrefix(strings.TrimSpace(s), "!")
	return nil
}

func (t altType) String() string {
	if t.v == "" {
		return ""
	}

	return "!" + t.v
}

func (t altType) StringSansOp() string { return t.v }

type altDigest struct{ v string }

func (d *altDigest) Set(s string) error {
	d.v = strings.TrimSpace(s)
	return nil
}

func (d altDigest) String() string { return d.v }

func (d altDigest) IsNull() bool { return d.v == "" }

func TestTypedBlobGenericNonDefaultInstantiation(t *testing.T) {
	coder := TypedMetadataCoder[altType, *altType, altDigest, *altDigest, struct{}]{}

	original := &TypedBlob[altType, *altType, altDigest, *altDigest, struct{}]{}
	if err := original.Type.Set("custom-type-v1"); err != nil {
		t.Fatal(err)
	}
	if err := original.BlobDigest.Set("blake2b256-deadbeef"); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	if _, err := coder.EncodeTo(original, writer); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(buf.String(), "! custom-type-v1") {
		t.Fatalf("expected type line in encoded output: %q", buf.String())
	}
	if !strings.Contains(buf.String(), "@ blake2b256-deadbeef") {
		t.Fatalf("expected digest line in encoded output: %q", buf.String())
	}

	decoded := &TypedBlob[altType, *altType, altDigest, *altDigest, struct{}]{}
	if _, err := coder.DecodeFrom(
		decoded,
		bufio.NewReader(strings.NewReader(buf.String())),
	); err != nil {
		t.Fatal(err)
	}

	if decoded.Type.String() != "!custom-type-v1" {
		t.Fatalf("type roundtrip mismatch: got %q", decoded.Type.String())
	}
	if decoded.BlobDigest.String() != "blake2b256-deadbeef" {
		t.Fatalf("digest roundtrip mismatch: got %q", decoded.BlobDigest.String())
	}
}
