---
author:
-
date: July 2026
title: HYPHENCE(1) Hyphence \| General Commands Manual
---

# NAME

hyphence - format-only inspection and re-emission of on-disk hyphence
documents

# SYNOPSIS

    hyphence [--no-inventory-log] COMMAND [ARG...]
    hyphence -h | --help
    hyphence COMMAND -h | --help

# DESCRIPTION

**hyphence** is format-only inspection and re-emission of on-disk hyphence
documents (RFC 0001 envelope; RFC 0002 content grammar). It has no
type-aware body decoding --- the four subcommands below work at the
metadata/body-envelope level only, streaming body bytes through
unexamined.

Every subcommand takes exactly one positional argument, **path**, which is
either a file path or **-** for stdin.

Running **hyphence** with no arguments, or **hyphence help** / **-h** /
**--help**, prints usage. **hyphence** *command* **-h** / **--help** prints
usage for that subcommand. An unrecognized subcommand is an error
(*unknown command: NAME*).

# GLOBAL OPTIONS

**--no-inventory-log**
:   Suppress the per-blob audit inventory-log under
    *\$XDG_LOG_HOME/madder/inventory_log/*. No-op for hyphence, which
    performs no blob writes --- kept for cross-utility flag-set consistency
    with madder-family tools sharing the same CLI framework.

# COMMANDS

**hyphence validate** *path|-*
:   Strict RFC 0001 conformance check. Silent and exits 0 on pass; on the
    first violation, prints one line-numbered diagnostic to stderr and
    exits non-zero. Also enforces the inline-body-AND-**@** rule (RFC 0001
    §Metadata Lines): a document must not carry both an **@** blob-reference
    line and a body section.

**hyphence meta** *path|-*
:   Print the metadata section to stdout, with the surrounding **---**
    boundaries stripped. No per-line validation runs --- malformed prefixes
    are printed through as-is. Boundary-level errors (missing closing
    **---**, missing body separator) still abort. Run **hyphence validate**
    first if strict checks matter.

**hyphence body** *path|-*
:   Stream the body section (the bytes after the closing **---** and the
    body separator) to stdout, verbatim. Prints nothing and exits 0 if the
    document has no body.

**hyphence format** *path|-*
:   Re-emit the document with metadata lines sorted per RFC 0002 §**<**
    Deprecation (amending RFC 0001 §Canonical Line Order): description
    (**#**) → reference, tag, and field lines (**-**, including the
    deprecated **<** synonym) → blob reference (**@**) → type (**!**).
    Within the **-**/**<** bucket, source order is preserved. Comments
    (**%**) stay anchored to their following non-comment line. Body bytes
    pass through unchanged. Idempotent --- formatting already-canonical
    input reproduces it byte-for-byte.

None of the four subcommands take subcommand-specific flags today; the only
flag surface is **--no-inventory-log**, above.

# EXIT STATUS

**0**
:   Success.

**non-zero**
:   Any failure: a validation violation, a malformed or unterminated
    document, a missing/unreadable input file, or an unrecognized
    subcommand. hyphence does not currently differentiate failure classes
    by exit code --- check stderr for the diagnostic.

# EXAMPLES

Validate a capture-receipt file against RFC 0001:

    hyphence validate receipt.hyphence

Print just the metadata section of a document:

    hyphence meta receipt.hyphence | grep '^!'

Pipe the body of an inventory-log file through jq:

    hyphence body $XDG_LOG_HOME/madder/inventory_log/2026-05-03/log.hyphence | jq -r '.entry_path'

Canonicalize an old document:

    hyphence format old.hyphence > canonical.hyphence

Read from stdin instead of a file:

    cat receipt.hyphence | hyphence validate -

# SEE ALSO

**hyphence**(7). For the normative specification, see
`docs/rfcs/0001-hyphence.md` (envelope) and `docs/rfcs/0002-content-grammar.md`
(content grammar).
