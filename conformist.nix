# hyphence's conformist config as a nix module, merged with
# conformist.lib.presets.eng in flake.nix (conformist.lib.evalModule). The
# preset enables the eng-convention linters (eng-versioning, flake-outputs/lock,
# the justfile-* roster); here we choose the formatters, the shellcheck linter,
# and the repo-specific tweaks. The generated config drives `nix fmt`
# (build.wrapper), the sandboxed checks.formatting (build.check), and the
# conformist-pre-commit hook (build.preCommit). See conformist-nix(7).
{ ... }:
{
  # Go: goimports (priority 1) before gofumpt (priority 2) so the import-grouped
  # output is re-canonicalized by gofumpt. Both pass -w (write in place).
  programs.goimports.enable = true;
  programs.goimports.priority = 1;
  programs.gofumpt.enable = true;
  programs.gofumpt.priority = 2;

  programs.nixfmt.enable = true;

  # shfmt over the shell we format (2-space indent, simplify, case-branch
  # indent, per the registry defaults). Narrow the includes to *.sh/*.bash/*.bats.
  programs.shfmt.enable = true;
  programs.shfmt.includes = [
    "*.sh"
    "*.bash"
    "*.bats"
  ];

  # just ships its own formatter; the registry wraps `just --fmt --unstable`
  # with a store-pinned `just`.
  programs.just.enable = true;

  # rustfmt formats the Rust impl (rust/hyphence/). conformist invokes rustfmt
  # per-file (not `cargo fmt`), so it reads the per-crate rustfmt.toml (which
  # pins edition = 2024) rather than Cargo.toml.
  programs.rustfmt.enable = true;

  # shellcheck (read-only linter) over the same shell set as shfmt.
  linters.shellcheck.enable = true;
  linters.shellcheck.includes = [
    "*.sh"
    "*.bash"
    "*.bats"
  ];

  # eng-versioning(7) derives the version key from go.mod's module path; hyphence's
  # module is github.com/amarbel-llc/hyphence/go, whose last segment yields the
  # wrong key (GO_VERSION). Pin it explicitly to match version.env.
  linters.eng-versioning.key = "HYPHENCE_VERSION";

  # Excludes layered on conformist's default-excludes (which already cover
  # *.lock, go.mod, go.sum, LICENSE). Only genuine prose/build/scratch artifacts
  # are excluded; version.env/sweatfile/gomod2nix.toml are deliberately NOT
  # excluded (the eng linters key on them, and no enabled formatter matches them).
  settings.excludes = [
    "*.md"
    "result"
    "result-*"
    ".tmp/**"
  ];
}
