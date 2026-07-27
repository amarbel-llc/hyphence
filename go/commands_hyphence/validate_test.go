package commands_hyphence

import (
	"errors"
	"strings"
	"testing"

	"code.linenisgreat.com/hyphence/go/hyphence"
)

func TestValidate_HappyPath(t *testing.T) {
	const input = "---\n! md\n---\n\nhello\n"
	if err := runValidate(strings.NewReader(input)); err != nil {
		t.Errorf("expected no error on valid input, got %v", err)
	}
}

func TestValidate_NoBody(t *testing.T) {
	const input = "---\n! md\n---\n"
	if err := runValidate(strings.NewReader(input)); err != nil {
		t.Errorf("expected no error on no-body document, got %v", err)
	}
}

func TestValidate_RejectsInlineBodyWithAt(t *testing.T) {
	const input = "---\n@ blake2b256-abc\n! md\n---\n\ninline\n"
	err := runValidate(strings.NewReader(input))
	if !errors.Is(err, hyphence.ErrInlineBodyWithAtReference) {
		t.Errorf("expected ErrInlineBodyWithAtReference, got %v", err)
	}
}

func TestValidate_RejectsInvalidPrefix(t *testing.T) {
	const input = "---\n! md\nX bad\n---\n"
	err := runValidate(strings.NewReader(input))
	if !errors.Is(err, hyphence.ErrInvalidPrefix) {
		t.Errorf("expected ErrInvalidPrefix, got %v", err)
	}
}

func TestValidate_RejectsMissingBodySeparator(t *testing.T) {
	const input = "---\n! md\n---\nhello\n" // no blank line after closing ---
	err := runValidate(strings.NewReader(input))
	if err == nil {
		t.Errorf("expected error for missing body separator, got nil")
	}
}

// TestValidate_RejectsNonBlech32Digest pins the behavior change
// hyphence#11 introduces at this surface: `validate` now checks each
// line's content against the RFC 0002/0003 content grammar, whose digest
// slot is charset-strict. 'b' is outside blech32, so this payload is
// refused — under the former permissive `'@' Ident` slot it passed.
func TestValidate_RejectsNonBlech32Digest(t *testing.T) {
	const input = "---\n! md@blake2b256-9bt3\n---\n"

	err := runValidate(strings.NewReader(input))
	if !errors.Is(err, hyphence.ErrMalformedContent) {
		t.Errorf("expected ErrMalformedContent, got %v", err)
	}
}

// TestValidate_AcceptsBlech32Digest is the positive twin of the above: the
// same shape with an in-alphabet payload must still pass, so the check
// rejects the charset violation rather than the markl-term shape.
func TestValidate_AcceptsBlech32Digest(t *testing.T) {
	const input = "---\n! md@blake2b256-9ft3x\n---\n"

	if err := runValidate(strings.NewReader(input)); err != nil {
		t.Errorf("expected no error on an in-alphabet digest, got %v", err)
	}
}

// TestValidate_RejectsQuotedPurposeWithBadDigest pins that the quote-aware
// purpose scan does not become an escape hatch: the join is the SECOND
// '@', so the digest slot is still `blake2b256-9bt3` and still refused.
func TestValidate_RejectsQuotedPurposeWithBadDigest(t *testing.T) {
	const input = "---\n! task\n- \"a@b\"@blake2b256-9bt3\n---\n"

	err := runValidate(strings.NewReader(input))
	if !errors.Is(err, hyphence.ErrMalformedContent) {
		t.Errorf("expected ErrMalformedContent, got %v", err)
	}
}

// TestMetadataValidator_ContentCheckIsOptIn guards the compatibility
// boundary from the other side. A retired pre-RFC-0003 spaced-lock
// spelling is envelope-valid but does NOT parse under the content
// grammar; with CheckContent off (the default, and what the library's
// decode path and document_test.go's document/parse-ok harness rely on)
// it must still be accepted. If this ever fails, the content check has
// leaked out of `validate` and broken RFC 0002's "existing decoders
// remain conforming" boundary.
func TestMetadataValidator_ContentCheckIsOptIn(t *testing.T) {
	const retiredSpelling = "---\n! md\n- one/uno @blake2b256-def\n---\n"

	lenient := &hyphence.MetadataValidator{}
	reader := hyphence.Reader{
		RequireMetadata: true,
		Metadata:        lenient,
		Blob:            &hyphence.CountingDiscardReaderFrom{},
	}

	if _, err := reader.ReadFrom(strings.NewReader(retiredSpelling)); err != nil {
		t.Errorf("default MetadataValidator rejected a retired-spelling document: %v", err)
	}

	// And the same document IS refused once the content pass is on,
	// confirming the field is what makes the difference.
	if err := runValidate(strings.NewReader(retiredSpelling)); !errors.Is(err, hyphence.ErrMalformedContent) {
		t.Errorf("with CheckContent set, expected ErrMalformedContent, got %v", err)
	}
}

// runValidate exercises the same Reader/consumer wiring as Validate.Run
// but takes a concrete reader and returns the error directly. The CLI
// wrapper (Validate.Run) handles printing the diagnostic and the
// futility-level cancellation; the validation logic itself is what we
// test here.
func runValidate(in *strings.Reader) error {
	return validateDocument(in)
}
