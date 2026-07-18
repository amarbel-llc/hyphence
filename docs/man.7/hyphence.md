---
author:
-
date: July 2026
title: HYPHENCE(7) Hyphence \| Miscellaneous
---

# NAME

hyphence - text-based metadata + body serialization format

# SYNOPSIS

    ---
    # description text
    - tag-name
    @ markl-id
    ! type-string
    ---

    body content

# DESCRIPTION

Hyphence (hyphen-fence) is a text-based serialization format that uses **---**
boundary lines to enclose a metadata section followed by an optional body. It
is used by madder for blob-store metadata and by cutting-garden for
capture-receipt metadata, and by dodder for
repository configs, blob store configs, workspace configs, type definitions,
and user-facing zettels.

# DOCUMENT STRUCTURE

A hyphence document has two parts:

1.  **Metadata section** (required): enclosed by two **---** boundary lines
2.  **Body section** (optional): content after the closing boundary

A minimal document:

    ---
    ! toml-type-v1
    ---

A document with inline body:

    ---
    # my description
    ! md
    ---

    Body content goes here.

A document with a blob reference (no inline body):

    ---
    # my description
    @ blake2b256-9ft3...
    ! md
    ---

A document must not have both a blob reference in the metadata and inline body
content after the closing boundary.

# BOUNDARY LINE

A boundary line is exactly three ASCII hyphen-minus characters followed by a
newline:

    ---\n

No trailing spaces, carriage returns, or additional hyphens.

# BODY SEPARATOR

When a body follows the metadata section, there must be exactly one empty line
between the closing **---** and the start of the body:

    ---
    ! md
    ---
                    <-- required blank line
    Body starts here.

Decoders MUST reject input that omits this blank line. Implementations MAY
expose an opt-in lenient mode for reading legacy data (see the
**AllowMissingSeparator** field on the Go reference implementation), but that
mode is NOT part of the format spec --- emitters MUST always include the
separator.

# METADATA LINES

Each metadata line begins with a single-character prefix, a space, and the
content:

**! type-string**
:   Object type identifier. Determines how the body is decoded. May include a
    lock, a space-separated **@digest**: **! type-string @markl-id**. Should
    be the last non-comment line.

**@ markl-id** or **@ file-path**
:   Blob reference. Alternative to inline body --- references content by
    digest or file path.

**# text**
:   Description. Multiple description lines are space-concatenated.

**- value**
:   Tag, object reference, or field. The value is an opaque identifier (or a
    quoted string, for content that would otherwise collide with reserved
    characters) --- there is no shape convention distinguishing tags from
    references. Whether a given value resolves to a tag or an object
    reference is decided by the type system when the document is consumed,
    not by whether the value contains a **/**; a value the type system can't
    resolve degrades to a plain tag. A value may carry a lock, a trailing
    space-separated **@digest**: **- value @markl-id** pins the reference to
    a specific content digest of the target as of write time.

    A value containing **=** is a *field line* instead of a tag/reference
    line: **- key=value**. A locked field line (**- key=value @markl-id**)
    names a **typed edge** --- **key** is the edge's label, **value** is the
    target identifier, and the digest pins the edge to the target's content
    as of write time. An id-less locked field (**- key=@markl-id**, digest
    immediately after **=**, no separate identifier) is an edge to a purely
    content-addressed target with no separate object identity. Field names
    beginning with **_** are reserved by the format for framework use (e.g.
    **_base**, **_mother**); type definitions must not declare a field name
    starting with **_**.

**< object-id** (deprecated)
:   A synonym for **-**, carried over from an earlier revision of the format
    where it overrode a shape convention that no longer exists. Decoders
    continue accepting **<** with the same content grammar as **-**, but
    encoders should emit **-** instead --- **<** has no remaining function
    distinct from it.

**% text**
:   Comment. Opaque, preserved during round-trips. Each comment is entangled
    with the non-comment line that follows it. Distinct from the *trailing*
    comment a structured line may carry after its own content --- see
    TRAILING COMMENTS below.

The grammar for what may appear after **- **/**< ** (identifiers, quoted
strings, field predicates, the **@digest** lock suffix) is specified formally
in `docs/rfcs/0002-content-grammar.md`, built on cutting-garden's trellis
query language.

# TRAILING COMMENTS

Metadata lines on the structured line kinds --- **!**, **@**, **-**, and the
deprecated **<** --- may carry a trailing comment: an inert annotation after
the line's complete content, introduced by a space and **%**:

    ! md % draft type, not yet stable
    - blocks=other/task @blake2b256-... % pinned at standup

A trailing comment is entangled with its **own** line --- the opposite of a
**%**-prefix comment line (above), which entangles with the line that
*follows* it. Content-preserving tools preserve trailing comments on
round-trip; a trailing comment carries no meaning of its own, it's pure
annotation.

A **%** inside a quoted value is content, not a comment boundary:

    - note="50% done" % real comment

parses as the field **note="50% done"** followed by the trailing comment
**% real comment** --- consumers parse the line's structure first and only
look for a trailing comment afterward, rather than splitting on the first
**%** they see.

Trailing comments are not recognized on **#** description lines (or on
description trailers in related surfaces like box format): free text runs to
end of line, and a description that happens to end in **%** would be
ambiguous with an intended comment.

# CANONICAL LINE ORDER

When encoding, metadata lines should follow this order:

1.  Description lines (**#**)
2.  Reference, tag, and field lines (**-**), in source order
3.  Blob line (**@**)
4.  Type line (**!**)

Lines may appear in any order during decoding --- order does not affect
semantics.

(Locked, aliased, and bare **-** lines used to sort into separate buckets
here; a lock is now a suffix any **-** line may carry rather than a distinct
line shape, so they share one bucket. Reference *aliasing* --- a future way
to give a reference a short local name --- has a syntax slot reserved for it
but is not yet specified.)

# VERSIONING

Hyphence has no version indicator. Format evolution uses type strings: new
versions introduce new type strings (e.g. **toml-blob_store_config-v2**
succeeds **-v1**) while old type strings retain their decoders. Old versions
remain decodable indefinitely.

# EXAMPLES

A zettel with tags:

    ---
    # purchase izipizi glasses
    - area-home
    - todo
    - urgency-2_week
    ! task
    ---

A blob store configuration:

    ---
    ! toml-blob_store_config-v3
    ---

    [blob-store]
    compression-type = "zstd"
    hash_type-id = "blake2b256"

A type definition with a lock:

    ---
    @ blake2b256-76m5...
    ! toml-type-v1 @ed25519_sig-1qxyz...
    ---

A task with a typed edge (a locked reference-valued field):

    ---
    # fix the thing
    - blocks=task/other-thing @blake2b256-9ft3...
    ! task
    ---

A repo config with an inline annotation:

    ---
    - compression-type=zstd % switched from gzip 2026-07
    ! toml-repo_config-v1
    ---

# SEE ALSO

**markl-id**(7), **organize-text**(7), **blob-store**(7). For the normative
envelope specification, see `docs/rfcs/0001-hyphence.md`; for the normative
content grammar (locks, field lines, typed edges, trailing comments, the
**<** deprecation), see `docs/rfcs/0002-content-grammar.md`.
