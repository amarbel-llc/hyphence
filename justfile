default: build test

[group('build')]
build: build-gomod2nix build-nix-check

# Regenerate go/gomod2nix.toml from go/go.mod + go/go.sum (the organic,
# non-bridged deps). The dewey require line is bridged via go/gomod.nix, but its
# gomod2nix.toml entry must still be present and current.
[group('build')]
build-gomod2nix:
    nix develop --command bash -c 'cd go && gomod2nix generate'

# Run the flake checks: the hermetic checks.go-test (go test -tags test ./...)
# and checks.formatting (the sandboxed conformist format + eng-linter gate).
# `nix build` does not evaluate `checks`, so this is the step that fires both.
[group('build')]
build-nix-check:
    nix flake check --show-trace

[group('post-build')]
test: test-go test-rust test-bats validate-grammar test-grammar-vectors lint-go lint-worktree

# Run the Go test suite in the devShell. CROWN JEWEL: the RFC conformance suite
# (go/hyphence/rfc_conformance_test.go) is behind //go:build test, so -tags test
# is mandatory or the conformance vectors silently don't run.
[group('post-build')]
test-go:
    nix develop --command go -C go test -tags test ./...

# Run the Rust test suite (unit tests + the RFC 0001 conformance harness) in the
# devShell. The crate is a member of the repo-root virtual workspace, so cargo
# runs from the repo root.
[group('post-build')]
test-rust:
    nix develop --command cargo test

# Run the hermetic CLI bats integration suite (zz-tests_bats/hyphence.bats) as a
# nix build against the built binary.
[group('post-build')]
test-bats:
    nix build .#bats-hyphence --show-trace

# Validate docs/rfcs/hyphence-content.peg (the formal RFC 0002/0003 companion,
# hyphence#7) with langlang, as a nix build against the check derivation.
# Grammar notation follows cutting-garden's "authored langlang-compatible"
# convention (docs/features/0022-trellis.md), but this is the first Nix-level
# langlang validation gate in either repo.
[group('post-build')]
validate-grammar:
    nix build .#validate-grammar --show-trace

# Cross-check RFC test vectors' content-grammar-governed metadata lines
# against docs/rfcs/hyphence-content.peg via langlang -input (hyphence#9):
# validate-grammar alone only checks the .peg is well-formed, never that
# real hyphence content parses under it.
[group('post-build')]
test-grammar-vectors:
    nix build .#grammar-vectors-test --show-trace

# Vet the Go sources — the cheap static-analysis pass.
[group('pre-build')]
lint-go:
    nix develop --command go -C go vet ./...

# Run the impure git-state eng-convention checks (git-remotes, git-default-branch,
# sweatfile, agents-md, gomod2nix) against the working tree, where .git and host
# tools are available — they can't run in the sandboxed checks.formatting.
[group('pre-build')]
lint-worktree:
    #!/usr/bin/env bash
    set -euo pipefail
    cfg=$(nix build --no-link --print-out-paths '.#conformist-impure-config')
    nix run '.#conformist' -- check --config-file "$cfg" --tree-root .

[group('codemod')]
codemod: codemod-fmt

# Format the whole tree in place via `nix fmt` (conformist repair mode): Go
# (goimports -> gofumpt), Nix (nixfmt), shell/bats (shfmt), justfile (just --fmt).
[group('codemod')]
codemod-fmt:
    nix fmt

[group('maintenance')]
update: update-go update-nix

# Tidy go/go.mod + go/go.sum, then regenerate go/gomod2nix.toml to match.
[group('maintenance')]
update-go: && build-gomod2nix
    nix develop --command go -C go mod tidy

# Update all flake inputs (flake.lock).
[group('maintenance')]
update-nix:
    nix flake update

# Rewrite version.env to the given semver (single source of truth per
# eng-versioning(7); flake.nix reads it via builtins.match). No-op if unchanged.
# Usage: just bump-version 0.2.0
[group('maintenance')]
bump-version new_version:
    #!/usr/bin/env bash
    set -euo pipefail
    current=""
    if [[ -f version.env ]]; then
      . ./version.env
      current="${HYPHENCE_VERSION:-}"
    fi
    if [[ "$current" == "{{ new_version }}" ]]; then
      echo "already at {{ new_version }}"
      exit 0
    fi
    printf 'export HYPHENCE_VERSION=%s\n' "{{ new_version }}" >version.env
    echo "bumped version: ${current:-(none)} -> {{ new_version }}"

# Tag a release. Pass the bare semver; the "v" prefix is added for you. Creates a
# signed annotated tag, pushes it to origin, then verifies the signature.
# Usage: just tag 0.1.0 "release v0.1.0"
[group('maintenance')]
tag version message:
    #!/usr/bin/env bash
    set -euo pipefail
    tag="v{{ version }}"
    prev=$(git tag --sort=-v:refname -l "v*" | head -1)
    if [[ -n "$prev" ]]; then
      echo "Previous: $prev"
      git log --oneline "$prev"..HEAD
    fi
    git tag -s -m "{{ message }}" "$tag"
    git push origin "$tag"
    git tag -v "$tag"

# Cut a release: must be run on master. Bumps version.env, commits the bump,
# pushes master, then signs and pushes the v{{ version }} tag.
# Usage: just release 0.1.0
[group('maintenance')]
release version:
    #!/usr/bin/env bash
    set -euo pipefail
    current_branch=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$current_branch" != "master" ]]; then
      echo "just release must be run on master (currently on $current_branch)" >&2
      exit 1
    fi
    prev=$(git tag --sort=-v:refname -l "v*" | head -1)
    header="release v{{ version }}"
    if [[ -n "$prev" ]]; then
      summary=$(git log --format='- %s' "$prev"..HEAD)
      msg="$header"$'\n\n'"$summary"
    else
      msg="$header"
    fi
    just bump-version "{{ version }}"
    if ! git diff --quiet version.env; then
      git add version.env
      git commit -m "chore: release v{{ version }}"
      git push origin master
    fi
    git tag -s -m "$msg" "v{{ version }}"
    git push origin "v{{ version }}"
