package hyphence

import (
	"bytes"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// These are the Go port of rust/hyphence/src/head.rs's unit tests: same
// inputs, same expectations, so a divergence between the two typed heads
// shows up as one of them failing a case the other passes.

func mustDigest(t *testing.T, s string) MarklDigest {
	t.Helper()

	digest, err := ParseMarklDigest(s)
	if err != nil {
		t.Fatalf("test digest %q: %v", s, err)
	}

	return digest
}

func ptr[T any](v T) *T { return &v }

func decodeHeadString(t *testing.T, input string) (Head, []byte, bool) {
	t.Helper()

	head, body, hasBody, err := DecodeHead(strings.NewReader(input))
	if err != nil {
		t.Fatalf("decode %q: %v", input, err)
	}

	return head, body, hasBody
}

func canonicalHead(t *testing.T, input string) string {
	t.Helper()

	head, body, hasBody := decodeHeadString(t, input)

	var out bytes.Buffer
	if err := head.Encode(&out, body, hasBody); err != nil {
		t.Fatalf("encode %q: %v", input, err)
	}

	return out.String()
}

func TestHead_ReadsEveryLineKind(t *testing.T) {
	input := "---\n" +
		"# a description\n" +
		"% entangled\n" +
		"- kind = root\n" +
		"- \"odd name\"=\"a \\\"q\\\" b\"\n" +
		"- key=piggy-piv_auth-v1@ssh_ecdsa_nistp256_pub-qqxyz\n" +
		"- _base=@blake2b256-jkl\n" +
		"- tag\n" +
		"< one/uno@blake2b256-def % pinned\n" +
		"@ 'a photo.png'\n" +
		"! md@blake2b256-acd\n" +
		"% tail\n" +
		"---\n"

	head, _, hasBody := decodeHeadString(t, input)

	if hasBody {
		t.Errorf("hasBody = true, want false")
	}

	want := []Line{
		{Kind: LineDescription, Text: "a description"},
		{Kind: LineField, Name: "kind", Value: Value{Text: "root"}},
		{Kind: LineField, Name: "odd name", Value: Value{Text: `a "q" b`}},
		{Kind: LineField, Name: "key", Value: Value{Markl: &MarklID{
			Purpose: ptr("piggy-piv_auth-v1"),
			Digest:  mustDigest(t, "ssh_ecdsa_nistp256_pub-qqxyz"),
		}}},
		{Kind: LineField, Name: "_base", Value: Value{Markl: &MarklID{
			Digest: mustDigest(t, "blake2b256-jkl"),
		}}},
		{Kind: LineTerm, Value: Value{Text: "tag"}},
		{Kind: LineTerm, Value: Value{Markl: &MarklID{
			Purpose: ptr("one/uno"),
			Digest:  mustDigest(t, "blake2b256-def"),
		}}},
		{Kind: LineBlob, Text: "a photo.png"},
		{Kind: LineType, Name: "md", Lock: ptr(mustDigest(t, "blake2b256-acd"))},
	}

	got := make([]Line, len(head.Lines))
	for i, line := range head.Lines {
		got[i] = line.Line
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("lines:\n got: %+v\nwant: %+v", got, want)
	}

	// A '%' line's text is what follows the prefix and its space; a
	// trailing comment's is everything after the '%'.
	if got := head.Lines[1].LeadingComments; !reflect.DeepEqual(got, []string{"entangled"}) {
		t.Errorf("leading comments of line 1 = %q, want [entangled]", got)
	}

	if got := head.Lines[6].TrailingComment; got == nil || *got != " pinned" {
		t.Errorf("trailing comment of line 6 = %v, want %q", got, " pinned")
	}

	if got := head.TrailingComments; !reflect.DeepEqual(got, []string{"tail"}) {
		t.Errorf("trailing comments = %q, want [tail]", got)
	}
}

func TestHead_CanonicalSpelling(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
	}{
		{
			"unspaced field re-emitted spaced",
			"---\n- due=\"2026-08-01\"\n---\n",
			"---\n- due = 2026-08-01\n---\n",
		},
		{
			"quoted value that can be bare",
			"---\n- state=\"open\"\n---\n",
			"---\n- state = open\n---\n",
		},
		{
			"single quotes re-emitted double",
			"---\n- note='a b'\n---\n",
			"---\n- note = \"a b\"\n---\n",
		},
		{
			"escapes round-trip",
			"---\n- note=\"a\\\"b\\\\c\\td\"\n---\n",
			"---\n- note = \"a\\\"b\\\\c\\td\"\n---\n",
		},
		{
			"a line feed in a value never breaks the line",
			"---\n- note=\"a\\nb\"\n---\n",
			"---\n- note = \"a\\nb\"\n---\n",
		},
		{
			"unknown escape collapses",
			"---\n- note=\"a\\qb c\"\n---\n",
			"---\n- note = \"aqb c\"\n---\n",
		},
		{
			"quoted purpose with the join rune",
			"---\n- \"a@b\"@blake2b256-xyz\n---\n",
			"---\n- \"a@b\"@blake2b256-xyz\n---\n",
		},
		{
			"quoted purpose that can be bare",
			"---\n- k=\"md\"@blake2b256-acd\n---\n",
			"---\n- k = md@blake2b256-acd\n---\n",
		},
		{
			"ident purpose outside PurposeBare gets quoted",
			"---\n- k=caldav:x@blake2b256-acd\n---\n",
			"---\n- k = \"caldav:x\"@blake2b256-acd\n---\n",
		},
		{
			"angle line emitted as dash",
			"---\n< blocks=other/task@blake2b256-ghj\n---\n",
			"---\n- blocks = other/task@blake2b256-ghj\n---\n",
		},
		{
			"prefix order and comments kept",
			"---\n! md\n% about the field\n- k=v%glued\n# d\n---\n\nbody\n",
			"---\n# d\n% about the field\n- k = v %glued\n! md\n---\n\nbody\n",
		},
		{
			"term-final sigil forces quoting",
			"---\n- \"todo:\"\n---\n",
			"---\n- \"todo:\"\n---\n",
		},
		{
			"empty text quoted",
			"---\n- k=\"\"\n---\n",
			"---\n- k = \"\"\n---\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := canonicalHead(t, tc.input); got != tc.want {
				t.Errorf("canonical:\n got: %q\nwant: %q", got, tc.want)
			}

			if got := canonicalHead(t, tc.want); got != tc.want {
				t.Errorf("not idempotent:\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// TestDecodeHead_EmptyBodyReadsAsNoBody pins a known divergence from the
// Rust Head::decode, which returns Some(empty) for the second input:
// Reader hands its Blob consumer the same empty stream for both, so
// DecodeHead cannot tell them apart. If Reader learns to, flip this test.
func TestDecodeHead_EmptyBodyReadsAsNoBody(t *testing.T) {
	for _, input := range []string{"---\n! md\n---\n", "---\n! md\n---\n\n"} {
		_, body, hasBody := decodeHeadString(t, input)
		if hasBody || len(body) != 0 {
			t.Errorf("%q: hasBody = %v, body = %q; want no body", input, hasBody, body)
		}
	}
}

func TestHead_EqualContentEqualBytes(t *testing.T) {
	a := canonicalHead(t, "---\n! t\n- k=\"v\"\n% c\n- j='w x'\n---\n")
	b := canonicalHead(t, "---\n- k = v\n% c\n- j = \"w x\"\n! t\n---\n")

	if a != b {
		t.Errorf("equal content encoded differently:\n a: %q\n b: %q", a, b)
	}
}

func TestHead_ConsumerOrdersWithinARank(t *testing.T) {
	head, _, _ := decodeHeadString(t, "---\n! t\n- b = 2\n- a = 1\n---\n")

	sort.SliceStable(head.Lines, func(i, j int) bool {
		x, y := head.Lines[i].Line, head.Lines[j].Line
		return x.Kind == LineField && y.Kind == LineField && x.Name < y.Name
	})

	var out bytes.Buffer
	if err := head.Encode(&out, nil, false); err != nil {
		t.Fatalf("encode: %v", err)
	}

	const want = "---\n- a = 1\n- b = 2\n! t\n---\n"
	if got := out.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHead_RefusesWhatTheGrammarRefuses(t *testing.T) {
	for _, input := range []string{
		"---\n! md @blake2b256-acd\n---\n",
		"---\n- tag(1)\n---\n",
		"---\n! md@blake2b256-9bt3\n---\n",
	} {
		_, _, _, err := DecodeHead(strings.NewReader(input))

		var contentErr *HeadContentError
		if !errors.As(err, &contentErr) {
			t.Errorf("%q: got %v, want a *HeadContentError", input, err)
			continue
		}

		if !errors.Is(err, ErrMalformedContent) {
			t.Errorf("%q: %v does not match ErrMalformedContent", input, err)
		}
	}
}

func TestHead_EncodeErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		head Head
		want error
	}{
		{
			"type with space",
			Head{Lines: []HeadLine{{Line: Line{Kind: LineType, Name: "a b"}}}},
			ErrTypeNameNotIdent,
		},
		{
			"purpose-less term",
			Head{Lines: []HeadLine{{Line: Line{Kind: LineTerm, Value: Value{Markl: &MarklID{
				Digest: mustDigest(t, "blake2b256-acd"),
			}}}}}},
			ErrPurposelessTerm,
		},
		{
			"multi-line description",
			Head{Lines: []HeadLine{{Line: Line{Kind: LineDescription, Text: "a\nb"}}}},
			ErrLineFeed,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.head.Encode(&bytes.Buffer{}, nil, false)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestMarklDigest_ChecksTheLexemeOnly(t *testing.T) {
	if _, err := ParseMarklDigest("blake2b256-9ft3x"); err != nil {
		t.Errorf("blake2b256-9ft3x: %v", err)
	}

	for _, s := range []string{"blake2b256-9bt3", "blake2b256-", "blake2b256"} {
		if _, err := ParseMarklDigest(s); !errors.Is(err, ErrInvalidDigest) {
			t.Errorf("ParseMarklDigest(%q) = %v, want ErrInvalidDigest", s, err)
		}
	}

	if _, err := NewMarklDigest("", "qq"); !errors.Is(err, ErrInvalidDigest) {
		t.Errorf("NewMarklDigest(\"\", \"qq\") = %v, want ErrInvalidDigest", err)
	}
}
