package hyphence

import (
	"errors"
	"testing"
)

// TestParseContent_Accepts pins the shapes RFC 0002/0003 admit. Each case
// names the production it exercises so a failure says which rule broke.
func TestParseContent_Accepts(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prefix  byte
		content string
	}{
		{"type/bare-ident", '!', "md"},
		{"type/markl-term", '!', "md@blake2b256-acd"},
		{"type/glued-trailing-comment", '!', "md%glued comment"},
		{"type/spaced-trailing-comment", '!', "md % spaced comment"},

		// BlobContent is String / Ident and deliberately does NOT go
		// through Digest: the '@' here is the line's PREFIX, not a
		// DigestTerm's term-initial '@', and the value may be a path. So
		// a 'b' that Digest would reject is fine in this position.
		{"blob/ident", '@', "blake2b256-abc"},
		{"blob/quoted-path", '@', "'pictures/a photo.png'"},

		{"dash/reference-markl-term", '-', "one/uno@blake2b256-def"},
		{"dash/field-quoted-value", '-', `due="2026-08-01"`},
		{"dash/field-spaced-equals", '-', `due = "2026-08-01"`},
		{"dash/field-bareword-value", '-', "state=open"},
		{"dash/id-less-field-lock", '-', "_base=@blake2b256-jkl"},
		{"dash/typed-edge", '-', "blocks=task/other@blake2b256-ghj"},
		{"dash/quoted-purpose", '-', `"my thing"@blake2b256-xyz`},
		{"dash/quoted-field-name", '-', `"odd name"=value`},
		{"dash/key-material-shape", '-', "piggy-piv_auth-v1@ssh_ecdsa_nistp256_pub-qqxyz"},

		// The '%' inside a quoted value is content, not a comment
		// boundary; the real comment follows it.
		{"dash/quoting-composes-with-comment", '-', `note="50% done" % real comment`},

		// A quoted purpose CONTAINING the join rune. The join is the
		// SECOND '@', which only falls out if the purpose slot is matched
		// quote-aware (piggy RFC 0011 §2.2).
		{"dash/quoted-purpose-with-join-rune", '-', `"a@b"@blake2b256-xyz`},

		// An interior sigil is identifier content because the rune after
		// it is itself identifier content — the &IdentRune lookahead's
		// positive case.
		{"dash/interior-sigil-is-ident", '-', "caldav:fastmail"},

		// '<' is a deprecated synonym of '-' with identical content
		// grammar.
		{"angle/field-predicate", '<', "blocks=other/task@blake2b256-def"},

		// Free text is unconditional within a line: reserved runes,
		// quotes and stray '@' are all just prose here.
		{"free-text/prose-with-reserved-runes", '#', `100% of "a@b" [done]`},
		{"comment/opaque", '%', `anything at all @ "` + "`"},
		{"free-text/empty", '#', ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ParseContent(tc.prefix, tc.content); err != nil {
				t.Errorf("ParseContent(%q, %q) = %v, want nil", string(tc.prefix), tc.content, err)
			}
		})
	}
}

// TestParseContent_Rejects pins what the charset-strict digest and the
// per-prefix productions refuse. The digest cases are the behavior change
// hyphence#11 introduces: every one of them parsed under the former
// permissive `'@' Ident` digest slot.
func TestParseContent_Rejects(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prefix  byte
		content string
	}{
		// 'b' is outside blech32, so it is an IdentRune sitting right
		// after the '9' DataChar run and the !IdentRune anchor refuses
		// rather than accepting a truncated '9'.
		{"digest/non-blech32-in-type", '!', "md@blake2b256-9bt3"},
		{"digest/non-blech32-in-field", '-', "pinned=other/thing@blake2b256-9bt3"},
		{"digest/non-blech32-in-digest-term", '-', "_base=@blake2b256-9bt3"},

		// blech32 is lowercase-only (piggy RFC 0011 §3.5): 'F' ends the
		// DataChar run and is then an IdentRune.
		{"digest/uppercase-data", '!', "md@BLAKE2B256-9FT3"},

		{"digest/missing-separator", '!', "md@blake2b256"},
		{"digest/empty-data", '!', "md@blake2b256-"},
		{"digest/missing-digest", '!', "md@"},
		{"digest/missing-format", '-', "_base=@-9ft3x"},

		{"string/unterminated", '-', `"unterminated`},
		{"type/unterminated-quote", '!', `"oops`},

		// Ident is IdentRune+ — one or more.
		{"type/empty", '!', ""},
		{"dash/empty", '-', ""},
		{"blob/empty", '@', ""},

		// Whitespace is not identifier content, so the second word is
		// unconsumed trailing input rather than part of the term.
		{"dash/unquoted-space", '-', "foo bar"},

		// A reserved rune cannot appear unquoted in a term.
		{"dash/reserved-rune-unquoted", '-', "a,b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ParseContent(tc.prefix, tc.content)
			if err == nil {
				t.Fatalf("ParseContent(%q, %q) = nil, want an error", string(tc.prefix), tc.content)
			}

			if !errors.Is(err, ErrMalformedContent) {
				t.Errorf("error %v does not match ErrMalformedContent", err)
			}

			var syntaxErr *ContentSyntaxError
			if !errors.As(err, &syntaxErr) {
				t.Fatalf("error %v is not a *ContentSyntaxError", err)
			}

			if syntaxErr.Prefix != tc.prefix {
				t.Errorf("Prefix = %q, want %q", string(syntaxErr.Prefix), string(tc.prefix))
			}

			if syntaxErr.Msg == "" {
				t.Error("Msg is empty; a diagnostic should say what was expected")
			}
		})
	}
}

// TestParseContent_InvalidPrefix keeps prefix validation distinct from
// content validation: an unknown prefix is not a content syntax error, it
// is the envelope-level ErrInvalidPrefix.
func TestParseContent_InvalidPrefix(t *testing.T) {
	err := ParseContent('X', "anything")
	if !errors.Is(err, ErrInvalidPrefix) {
		t.Errorf("ParseContent('X', ...) = %v, want ErrInvalidPrefix", err)
	}

	if errors.Is(err, ErrMalformedContent) {
		t.Error("an invalid prefix should not report as malformed content")
	}
}

// TestIsContentIdentRuneAt_StrictSigilRule pins the recursive &IdentRune
// lookahead directly: a sigil rune is identifier content only when what
// follows it is, so a term-final sigil (or a run of them) is not.
func TestIsContentIdentRuneAt_StrictSigilRule(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		at   int
		want bool
	}{
		{"interior-sigil-followed-by-ident", "a:b", 1, true},
		{"term-final-sigil", "a:", 1, false},
		{"trailing-sigil-run-bottoms-out", "a::", 1, false},
		{"sigil-run-then-ident", "a::b", 1, true},
		{"hyphen-is-unconditional", "a-", 1, true},
		{"slash-is-unconditional", "a/", 1, true},
		{"reserved-rune", "a@b", 1, false},
		{"whitespace", "a b", 1, false},
		{"plain-rune", "ab", 1, true},
		{"out-of-range-high", "a", 1, false},
		{"out-of-range-low", "a", -1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isContentIdentRuneAt([]rune(tc.in), tc.at); got != tc.want {
				t.Errorf("isContentIdentRuneAt(%q, %d) = %v, want %v", tc.in, tc.at, got, tc.want)
			}
		})
	}
}

// TestIsContentDataChar_OmitsAmbiguousRunes pins the four characters
// bech32 drops as visually ambiguous. These are exactly what separates the
// strict digest from the former permissive identifier slot, so a
// regression here would silently restore the old laxness.
func TestIsContentDataChar_OmitsAmbiguousRunes(t *testing.T) {
	for _, r := range []rune{'b', 'i', 'o', '1'} {
		if isContentDataChar(r) {
			t.Errorf("isContentDataChar(%q) = true, want false (outside blech32)", r)
		}
	}

	const blech32 = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	for _, r := range blech32 {
		if !isContentDataChar(r) {
			t.Errorf("isContentDataChar(%q) = false, want true (in blech32)", r)
		}
	}

	if got := len(blech32); got != 32 {
		t.Errorf("blech32 alphabet has %d characters, want 32", got)
	}
}
