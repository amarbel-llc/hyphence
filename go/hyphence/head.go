package hyphence

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
)

// This file is the typed head: a hyphence document's metadata section
// read into typed lines (RFC 0002 as amended by RFC 0003), and the
// canonical encoder that writes it back (RFC 0005,
// docs/rfcs/0005-canonical-emission.md). It is the Go twin of
// rust/hyphence/src/head.rs; the head/canonical vectors in
// testdata/rfc_vectors.txt keep the two encoders byte-identical.
//
// Document keeps every line's content as an opaque string, which is all
// RFC 0001 needs. Head is the layer above it: each line parsed by the
// production its prefix selects (content.go's parseLine), every quoted
// string unescaped, every markl id split into its purpose and digest
// slots. A consumer reads a head, reorders or edits its lines, and
// encodes it; two heads with equal typed content encode to equal bytes.
//
// WHAT STAYS OUT
//
//   - Markl decoding. A MarklDigest is TEXT: hyphence checks the lexeme
//     (format runes, a blech32-alphabet payload) and never decodes blech32
//     or checks a length or checksum. That is piggy-markl's job.
//   - The body. Bytes, untouched. Its meaning is the '!' type's concern.
//   - Field tables. Which fields a type requires, their order, set
//     semantics, refusals: the consumer's. Hyphence orders lines by prefix
//     rank only, keeping the consumer's order within a rank.
//
// CANONICAL EMISSION
//
//   - prefix order '#', '-', '@', '!' (RFC 0002 §`<` Deprecation), stable
//     (Canonicalize), so lines of one rank keep their order in Head.Lines;
//   - '<' is never emitted: a '<' line reads as a '-' line and is written
//     '-';
//   - field lines spaced, `- key = value` (RFC 0003 §Canonical emission);
//   - a text (field name, value, tag or reference, blob path) bare when it
//     is a valid Ident, otherwise double-quoted with Quote's escapes;
//   - a markl id's purpose bare when it fits markl's own PurposeBare
//     ([a-zA-Z0-9_/-]+, piggy marklid.peg), otherwise quoted, joined by '@'
//     right after the closing quote; a purpose-less markl id in a field
//     value is the id-less DigestTerm, `@format-data`;
//   - comments KEPT: '%' lines before the line they are entangled with,
//     and a trailing comment as ` %text` after the content. A consumer
//     that wants them gone removes them from the model first.

var (
	// ErrInvalidDigest is returned by NewMarklDigest and ParseMarklDigest when the
	// format or data is outside its charset or empty.
	ErrInvalidDigest = errors.New("not a markl-id digest")

	// ErrTypeNameNotIdent is an encode error: an unlocked type name that
	// is not an Ident. TypeContent admits a quoted name only as a locked
	// markl id's purpose.
	ErrTypeNameNotIdent = errors.New("unlocked type name is not an identifier")

	// ErrPurposelessTerm is an encode error: a tag/reference term that is
	// a markl id with no purpose. GroundTerm has no DigestTerm alternative.
	ErrPurposelessTerm = errors.New("a tag/reference line cannot hold a purpose-less markl id")

	// ErrLineFeed is an encode error: a description or comment containing
	// a line feed, which would end the line early.
	ErrLineFeed = errors.New("line feed inside free text")
)

// MarklDigest is a markl-id digest slot, `format-data`, as text:
// Format '-' DataChar+. (It is the Rust head's Digest; the name Digest is
// taken in this package by the opaque `@`-line value in type_digest.go.)
//
// Construction checks the lexeme only — format runes [a-zA-Z0-9_], a
// non-empty payload in the blech32 alphabet. It is length-agnostic and
// never decodes the payload. The zero MarklDigest is not a valid digest.
type MarklDigest struct {
	format, data string
}

// NewMarklDigest builds a digest from its format id and its data payload.
// Errors match errors.Is(err, ErrInvalidDigest).
func NewMarklDigest(format, data string) (MarklDigest, error) {
	if !isDigestParts(format, data) {
		return MarklDigest{}, fmt.Errorf(
			"%w: format %q, data %q (format must be [a-zA-Z0-9_]+, data blech32)",
			ErrInvalidDigest, format, data,
		)
	}

	return MarklDigest{format: format, data: data}, nil
}

// ParseMarklDigest parses the `format-data` spelling. The split is at the
// first '-', since a format id never contains one.
func ParseMarklDigest(s string) (MarklDigest, error) {
	format, data, _ := strings.Cut(s, "-")
	return NewMarklDigest(format, data)
}

// Format is the format id, e.g. "blake2b256".
func (d MarklDigest) Format() string { return d.format }

// Data is the blech32-alphabet payload, undecoded.
func (d MarklDigest) Data() string { return d.data }

// String is the `format-data` spelling.
func (d MarklDigest) String() string { return d.format + "-" + d.data }

// MarklID is a markl id as text: an optional purpose joined to a digest
// (`[purpose@]format-data`, piggy RFC 0011 §2).
type MarklID struct {
	// Purpose is UNESCAPED (`"a@b"@…` reads as `a@b`). nil is a
	// purpose-less id, which hyphence content admits only as a field's
	// id-less DigestTerm (`- _base = @blake2b256-…`).
	Purpose *string
	Digest  MarklDigest
}

// Value is a field value, or the term of a tag/reference line: a markl id
// when Markl is non-nil, otherwise the text in Text.
//
// A bare word and a quoted string are ONE kind here: both are text, and
// the encoder chooses the spelling (bare when the grammar allows it). So
// `"open"` and `open` are equal content.
type Value struct {
	Markl *MarklID
	Text  string
}

// LineKind discriminates Line.
type LineKind int

const (
	// LineDescription is `#` free text, verbatim in Line.Text (no
	// trimming, no merging).
	LineDescription LineKind = iota
	// LineField is `- name = value` (RFC 0002 FieldContent): Line.Name
	// (unescaped) and Line.Value.
	LineField
	// LineTerm is `- term`, a tag or a reference, in Line.Value (RFC 0002
	// deleted the '/'-shape rule, so hyphence does not tell them apart). A
	// markl term is a locked reference and always has a purpose.
	LineTerm
	// LineBlob is the `@` blob reference: a markl id or a path, as text in
	// Line.Text (BlobContent).
	LineBlob
	// LineType is the `!` type: Line.Name, optionally locked by Line.Lock
	// (`! md@blake2b256-…`).
	LineType
)

// Line is one metadata line's typed content. Which fields are meaningful
// depends on Kind (see the LineKind constants).
type Line struct {
	Kind LineKind
	// Text is the description (LineDescription) or blob path (LineBlob).
	Text string
	// Name is the field name (LineField) or type name (LineType).
	Name string
	// Value is the field value (LineField) or the term (LineTerm).
	Value Value
	// Lock is the type lock (LineType); nil for an unlocked type.
	Lock *MarklDigest
}

// HeadLine is a typed line with the comments entangled with it.
type HeadLine struct {
	Line Line
	// LeadingComments are the '%' lines immediately before this one (RFC
	// 0001: entangled with the next non-comment line, so they move with
	// it).
	LeadingComments []string
	// TrailingComment is the text after a trailing '%' on this line (RFC
	// 0002), verbatim; nil when there is none.
	TrailingComment *string
}

// Head is a document's metadata section as typed lines.
type Head struct {
	// Lines are in source order. The encoder sorts by prefix rank only, so
	// this order is kept within a rank: a consumer orders its fields by
	// ordering this slice.
	Lines []HeadLine
	// TrailingComments are the '%' lines after the last non-comment line.
	TrailingComments []string
}

// HeadContentError reports a metadata line whose content does not parse
// under its prefix's production. It unwraps to the content error, so
// errors.Is(err, ErrMalformedContent) matches.
type HeadContentError struct {
	// Line is the index into Document.Metadata (comments not counted).
	Line int
	Err  error
}

func (e *HeadContentError) Error() string {
	return fmt.Sprintf("metadata line %d: %v", e.Line, e.Err)
}

func (e *HeadContentError) Unwrap() error { return e.Err }

// HeadFromDocument reads every metadata line of doc as typed content. '<'
// lines read as '-' lines. It fails on the first line the content grammar
// refuses, including the retired RFC 0002 spaced-lock spelling, with a
// *HeadContentError naming the line.
func HeadFromDocument(doc *Document) (Head, error) {
	head := Head{
		Lines:            make([]HeadLine, 0, len(doc.Metadata)),
		TrailingComments: doc.TrailingComments,
	}

	for index, metadata := range doc.Metadata {
		parsed, err := parseLine(metadata.Prefix, metadata.Value)
		if err != nil {
			return Head{}, &HeadContentError{Line: index, Err: err}
		}

		// MetadataBuilder never yields a '%' metadata line (comments ride
		// on LeadingComments), so a typed line is always present here.
		if parsed.isComment {
			continue
		}

		head.Lines = append(head.Lines, HeadLine{
			Line:            parsed.line,
			LeadingComments: metadata.LeadingComments,
			TrailingComment: parsed.trailingComment,
		})
	}

	return head, nil
}

// DecodeHead decodes a document into a typed head and its untouched body,
// with the strict blank-separator rule. Input with no opening boundary is
// all body and an empty head, as RFC 0001 reads it.
//
// hasBody reports whether the document had a body section: true for
// "---\n…\n---\n\n" with an empty body, false for "---\n…\n---\n", the
// same split as Rust's Head::decode returning Some(empty) or None.
func DecodeHead(r io.Reader) (head Head, body []byte, hasBody bool, err error) {
	var doc Document
	var bodyBuffer bytes.Buffer

	reader := Reader{
		Metadata: &MetadataBuilder{Doc: &doc},
		Blob:     &BodyStreamer{W: &bodyBuffer},
	}

	if _, err = reader.ReadFrom(r); err != nil {
		return Head{}, nil, false, err
	}

	if head, err = HeadFromDocument(&doc); err != nil {
		return Head{}, nil, false, err
	}

	if !reader.HasBody() {
		return head, nil, false, nil
	}

	return head, bodyBuffer.Bytes(), true, nil
}

// Document returns the canonical Document for this head: every line
// spelled canonically, in Lines order (Encode then orders them with
// Canonicalize). HasBody is left false; the body is Encode's concern.
func (h Head) Document() (Document, error) {
	doc := Document{
		Metadata:         make([]MetadataLine, 0, len(h.Lines)),
		TrailingComments: h.TrailingComments,
	}

	for _, line := range h.Lines {
		prefix, value, err := spellLine(line.Line)
		if err != nil {
			return Document{}, err
		}

		if line.TrailingComment != nil {
			if err := checkNoLineFeed(*line.TrailingComment); err != nil {
				return Document{}, err
			}

			value += " %" + *line.TrailingComment
		}

		for _, comment := range line.LeadingComments {
			if err := checkNoLineFeed(comment); err != nil {
				return Document{}, err
			}
		}

		doc.Metadata = append(doc.Metadata, MetadataLine{
			Prefix:          prefix,
			Value:           value,
			LeadingComments: line.LeadingComments,
		})
	}

	for _, comment := range h.TrailingComments {
		if err := checkNoLineFeed(comment); err != nil {
			return Document{}, err
		}
	}

	return doc, nil
}

// Encode writes the canonical bytes for this head and body to w.
// hasBody with an empty body writes the blank separator line with nothing
// after it; !hasBody ends the document at the closing boundary. A head
// with no lines and no comments is written as its body alone, the same as
// an RFC 0001 document with no metadata section.
//
// Errors are ErrTypeNameNotIdent, ErrPurposelessTerm or ErrLineFeed
// (wrapped) when a line has no spelling, or w's write error.
func (h Head) Encode(w io.Writer, body []byte, hasBody bool) error {
	doc, err := h.Document()
	if err != nil {
		return err
	}

	Canonicalize(&doc)

	bw := bufio.NewWriter(w)

	if len(doc.Metadata) > 0 || len(doc.TrailingComments) > 0 {
		bw.WriteString(Boundary + "\n")

		for _, ml := range doc.Metadata {
			for _, comment := range ml.LeadingComments {
				writeMetadataLine(bw, '%', comment)
			}

			writeMetadataLine(bw, ml.Prefix, ml.Value)
		}

		for _, comment := range doc.TrailingComments {
			writeMetadataLine(bw, '%', comment)
		}

		bw.WriteString(Boundary + "\n")

		if hasBody {
			bw.WriteByte('\n')
		}
	}

	if hasBody {
		bw.Write(body)
	}

	return bw.Flush()
}

func writeMetadataLine(bw *bufio.Writer, prefix byte, content string) {
	bw.WriteByte(prefix)
	bw.WriteByte(' ')
	bw.WriteString(content)
	bw.WriteByte('\n')
}

// Field is one `- name = value` line's content.
type Field struct {
	Name  string
	Value Value
}

// Fields returns the `- name = value` lines, in order.
func (h Head) Fields() []Field {
	var fields []Field

	for _, line := range h.Lines {
		if line.Line.Kind == LineField {
			fields = append(fields, Field{Name: line.Line.Name, Value: line.Line.Value})
		}
	}

	return fields
}

// TypeLine returns the first '!' type line's name and lock (nil when
// unlocked); ok is false when the head has no type line.
func (h Head) TypeLine() (name string, lock *MarklDigest, ok bool) {
	for _, line := range h.Lines {
		if line.Line.Kind == LineType {
			return line.Line.Name, line.Line.Lock, true
		}
	}

	return "", nil, false
}

func checkNoLineFeed(text string) error {
	if strings.ContainsRune(text, '\n') {
		return fmt.Errorf("%w: %q", ErrLineFeed, text)
	}

	return nil
}

// spellLine returns one line's prefix and canonical content (without any
// trailing comment).
func spellLine(line Line) (byte, string, error) {
	switch line.Kind {
	case LineDescription:
		if err := checkNoLineFeed(line.Text); err != nil {
			return 0, "", err
		}

		return '#', line.Text, nil

	case LineField:
		return '-', spellText(line.Name) + " = " + spellValue(line.Value), nil

	case LineTerm:
		if line.Value.Markl != nil && line.Value.Markl.Purpose == nil {
			return 0, "", fmt.Errorf("%w: @%s", ErrPurposelessTerm, line.Value.Markl.Digest)
		}

		return '-', spellValue(line.Value), nil

	case LineBlob:
		return '@', spellText(line.Text), nil

	case LineType:
		if line.Lock == nil {
			if !isBareIdent(line.Name) {
				return 0, "", fmt.Errorf("%w: %q", ErrTypeNameNotIdent, line.Name)
			}

			return '!', line.Name, nil
		}

		return '!', spellPurpose(line.Name) + "@" + line.Lock.String(), nil

	default:
		return 0, "", fmt.Errorf("unknown line kind %d", line.Kind)
	}
}

func spellValue(value Value) string {
	switch {
	case value.Markl == nil:
		return spellText(value.Text)

	case value.Markl.Purpose == nil:
		return "@" + value.Markl.Digest.String()

	default:
		return spellPurpose(*value.Markl.Purpose) + "@" + value.Markl.Digest.String()
	}
}

// spellText writes a text bare when it is a valid Ident, else quoted.
func spellText(text string) string {
	if isBareIdent(text) {
		return text
	}

	return Quote(text)
}

// spellPurpose writes a markl purpose bare when it fits marklid.peg's
// PurposeBare ([a-zA-Z0-9_/-]+), else quoted — the same rule as
// piggy-markl's spell_purpose.
func spellPurpose(purpose string) string {
	if isPurposeBare(purpose) {
		return purpose
	}

	return Quote(purpose)
}

func isPurposeBare(purpose string) bool {
	if purpose == "" {
		return false
	}

	for _, r := range purpose {
		if !isContentFormatChar(r) && r != '/' && r != '-' {
			return false
		}
	}

	return true
}

// Quote double-quotes text with the Doddish escapes piggy-markl's
// quote_purpose writes: \\ \" \n \t \r \a \b \f \v; every other rune is
// itself.
func Quote(text string) string {
	var out strings.Builder
	out.Grow(len(text) + 2)
	out.WriteByte('"')

	for _, r := range text {
		switch r {
		case '\\':
			out.WriteString(`\\`)
		case '"':
			out.WriteString(`\"`)
		case '\n':
			out.WriteString(`\n`)
		case '\t':
			out.WriteString(`\t`)
		case '\r':
			out.WriteString(`\r`)
		case '\a':
			out.WriteString(`\a`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\v':
			out.WriteString(`\v`)
		default:
			out.WriteRune(r)
		}
	}

	out.WriteByte('"')

	return out.String()
}
