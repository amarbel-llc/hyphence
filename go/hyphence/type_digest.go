package hyphence

import "strings"

// Type is the hyphence document type tag — the value carried on the `!`
// metadata line (see RFC 0001 §Metadata Line Types). Hyphence treats the tag as
// opaque and round-trips it verbatim; it imposes no scheme on the tag's
// contents. The leading `!` operator is not stored: String renders the tag with
// the operator, StringSansOp without it.
//
// Type is a value type whose zero value is the empty (unset) tag, so
// `var t Type; t.Set(...)` and `&TypedBlob[B]{}` followed by `.Type.Set(...)`
// both work without further initialization.
type Type struct {
	value string
}

// Set assigns the tag value. A leading `!` operator and surrounding whitespace
// are stripped, so both "inventory_list-v2" and "! inventory_list-v2" yield the
// same tag. It never returns an error; the signature satisfies the dewey
// FuncSetString contract used by the metadata line reader.
func (t *Type) Set(value string) error {
	t.value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "!"))
	return nil
}

// String renders the tag with its `!` operator (e.g. "!inventory_list-v2"), or
// the empty string when unset.
func (t Type) String() string {
	if t.value == "" {
		return ""
	}

	return "!" + t.value
}

// StringSansOp renders the tag without the `!` operator (e.g.
// "inventory_list-v2"). The encoder uses this to emit the `! <tag>` line.
func (t Type) StringSansOp() string {
	return t.value
}

// Digest is the hyphence blob-digest reference — the value carried on the `@`
// metadata line (see RFC 0001 §Metadata Line Types). Hyphence treats it as
// opaque and round-trips it verbatim; the concrete digest scheme (e.g. a
// markl-id such as "blake2b256-…") is the caller's concern, not the format's.
//
// Digest is a value type whose zero value is null (IsNull reports true), so a
// freshly constructed TypedBlob has no `@` line until Set is called.
type Digest struct {
	value string
}

// Set assigns the digest value, trimming surrounding whitespace. It never
// returns an error; the signature satisfies the dewey FuncSetString contract.
func (d *Digest) Set(value string) error {
	d.value = strings.TrimSpace(value)
	return nil
}

// String renders the digest verbatim, or the empty string when null.
func (d Digest) String() string {
	return d.value
}

// IsNull reports whether the digest is unset. The encoder omits the `@` line
// for a null digest.
func (d Digest) IsNull() bool {
	return d.value == ""
}
