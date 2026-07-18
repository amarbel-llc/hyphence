package hyphence

import "sort"

// canonicalOrder maps each metadata prefix to its sort rank per RFC
// 0002 §`<` Deprecation (amending RFC 0001 §Canonical Line Order).
// Lower rank emits first. The map also serves as the single source
// of truth for the metadata prefix alphabet — isValidPrefix derives
// membership from it.
//
// Comments ('%') are listed with sentinel rank -1 ("filter-out"):
// MetadataBuilder captures them as anchored LeadingComments /
// TrailingComments rather than as standalone MetadataLine entries,
// so Canonicalize never sees them in practice. The sentinel rank
// keeps the map authoritative for the alphabet without disturbing
// sort behavior — if a '%' ever did slip through, it would sort
// first, which is harmless.
//
// '<' shares '-'s rank: RFC 0002 deprecates '<' to a synonym of '-',
// and its side effect on canonical order is that the previously
// separate locked/aliased/bare object-reference rank (issue #128)
// collapses into one bucket alongside tags and field lines — a lock
// is now a suffix property of a '-' line, not a distinct line shape
// or PREFIX choice. Stable sort preserves source order within the
// merged bucket.
var canonicalOrder = map[byte]int{
	'%': -1, // comment (handled separately by Builder; never appears as a top-level MetadataLine)
	'#': 0,  // description
	'-': 1,  // reference, tag, and field lines
	'<': 1,  // deprecated synonym of '-' (RFC 0002 §`<` Deprecation)
	'@': 2,  // blob reference
	'!': 3,  // type
}

// isValidPrefix reports whether b is a recognized metadata-line
// prefix per RFC 0001 §Metadata Lines. The alphabet is whatever
// canonicalOrder lists.
func isValidPrefix(b byte) bool {
	_, ok := canonicalOrder[b]
	return ok
}

// Canonicalize sorts doc.Metadata in place per RFC 0001 §Canonical
// Line Order. Stable sort within each prefix bucket preserves
// insertion order. Each MetadataLine carries its LeadingComments
// across the sort. TrailingComments remain at the document tail.
func Canonicalize(doc *Document) {
	sort.SliceStable(doc.Metadata, func(i, j int) bool {
		return canonicalOrder[doc.Metadata[i].Prefix] < canonicalOrder[doc.Metadata[j].Prefix]
	})
}
