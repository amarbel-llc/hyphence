{
  description = "hyphence — the hyphen-fence document format (text-based metadata + body serialization)";

  inputs = {
    # amarbel-llc/nixpkgs fork. Pre-bundles buildGoApplication / mkGoEnv (the
    # gomod2nix overlay, with goFlakeInputs support per nixpkgs RFC 0001) into
    # the base pkgs set, so this flake's build closure matches madder's.
    igloo = {
      url = "https://code.linenisgreat.com/igloo/archive/master.tar.gz";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
    };

    # SHA-pinned upstream anchor (Hydra-blessed, cache.nixos.org-served).
    # Sourced as pkgs-master in go/default.nix for go_1_26 + shell tools.
    nixpkgs-master.url = "github:NixOS/nixpkgs/567a49d1913ce81ac6e9582e3553dd90a955875f";

    utils = {
      url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
      inputs.systems.follows = "igloo/systems";
    };

    # bats: amarbel-llc/bats provides lib.batsLane for future CLI/integration
    # lanes. No bats suite yet (library-only); not consumed in an output yet.
    bats = {
      url = "https://code.linenisgreat.com/bats/archive/master.tar.gz";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.igloo.follows = "igloo";
      inputs.utils.follows = "utils";
    };

    # purse-first: dewey's home. Sourced via go/gomod.nix's goFlakeInputs to
    # bridge github.com/amarbel-llc/purse-first/libs/dewey, so a dewey bump is
    # a flake.lock-only edit (RFC 0001 § Consumer interface).
    purse-first = {
      url = "https://code.linenisgreat.com/purse-first/archive/master.tar.gz";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.igloo.follows = "igloo";
      inputs.utils.follows = "utils";
      inputs.conformist.follows = "conformist";
    };

    # conformist: the linter/formatter multiplexer (treefmt successor). Config
    # is Nix-generated from ./conformist.nix + presets.eng via evalModule.
    conformist = {
      url = "https://code.linenisgreat.com/conformist/archive/master.tar.gz";
      inputs.igloo.follows = "igloo";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };

    # doppelgang provides `lint`, the flake.lock dedup gate. On the devShell
    # PATH; the follows wiring above keeps one node per shared input.
    doppelgang = {
      url = "https://code.linenisgreat.com/doppelgang/archive/master.tar.gz";
      inputs.igloo.follows = "igloo";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };
    doppelgang.inputs.conformist.follows = "conformist";
  };

  outputs =
    {
      self,
      igloo,
      nixpkgs-master,
      utils,
      bats,
      purse-first,
      conformist,
      doppelgang,
      ...
    }:
    let
      # version.env is the single source of truth for the release version
      # (eng-versioning(7)). The match captures everything after
      # HYPHENCE_VERSION= up to the line break; the `export` prefix is tolerated.
      hyphenceVersion = builtins.head (
        builtins.match ".*HYPHENCE_VERSION=([^\n]+).*" (builtins.readFile ./version.env)
      );
    in
    utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import igloo { inherit system; };

        # gomod.nix is the mixed producer+consumer half of the flake-input-go_mod
        # protocol: goFlakeInputs (consumer) bridges dewey; goPkgs (producer)
        # publishes go-pkgs / go-pkgs-test so downstream repos (madder) can bridge
        # hyphence as a flake input. The producer src is scoped to the go/ subdir,
        # so downstream bridges with NO subPath.
        gomod = import ./go/gomod.nix {
          inherit pkgs purse-first system;
          src = self + "/go";
        };
        inherit (gomod) goFlakeInputs;
        inherit (gomod.goPkgs) go-pkgs go-pkgs-test;

        # Pure lane: the eng preset (sandboxed eng-convention linters) + this
        # repo's formatters/excludes. Drives `nix fmt` (build.wrapper), the
        # sandboxed checks.formatting (build.check), and the
        # conformist-pre-commit hook (build.preCommit).
        conformistEval = conformist.lib.evalModule pkgs {
          imports = [
            conformist.lib.presets.eng
            ./conformist.nix
          ];
          package = conformist.packages.${system}.default;
        };

        # Impure lane: the git-state eng-convention checks (git-remotes,
        # git-default-branch, sweatfile, agents-md, gomod2nix). They need a live
        # .git / host tools, so they run against the working tree via `just
        # lint-worktree`, exposed below as packages.conformist-impure-config.
        conformistImpureEval = conformist.lib.evalModule pkgs {
          imports = [
            conformist.lib.presets.eng-impure
            # clippy (conformist#69) is opt-in (not in the eng-impure roster).
            # Enable it for the rust/ crate. workspace = true because the repo
            # root is a VIRTUAL workspace (no root [package]); the default
            # toolchain (igloo's cargo/clippy/rustc) builds edition 2024, matching
            # the rustPlatform that builds checks.rust-test.
            {
              linters.clippy.enable = true;
              linters.clippy.workspace = true;
            }
          ];
          package = conformist.packages.${system}.default;
          projectRootFile = "flake.nix";
        };

        result = import ./go/default.nix {
          nixpkgs = igloo;
          inherit
            nixpkgs-master
            conformist
            doppelgang
            system
            goFlakeInputs
            ;
          version = hyphenceVersion;
          conformistPreCommit = conformistEval.config.build.preCommit;
          # The merge-repair sibling, on the devShell PATH as
          # `conformist-repair` so spinclass's repair phase self-heals bump
          # commits with this repo's own config (eng tier-B convergence).
          conformistRepair = conformistEval.config.build.repair;
          man7Src = ./docs/man.7;
        };

        # bats-hyphence: the hermetic CLI integration lane (zz-tests_bats/
        # hyphence.bats drives the built binary via HYPHENCE_BIN). Built as a
        # package (`nix build .#bats-hyphence`, run by `just test-bats`); success
        # leaves a stamp at $out, failure aborts with the bats diagnostic.
        bats-hyphence = bats.lib.${system}.batsLane {
          base = result.packages.hyphence;
          batsSrc = ./zz-tests_bats;
          binaries = {
            HYPHENCE_BIN = {
              base = result.packages.hyphence;
              name = "hyphence";
            };
          };
          batsLibPath = [ bats.packages.${system}.bats-libs.batsLibPath ];
          # timeout (coreutils) + cmp (diffutils) are used by the idempotency test.
          nativeBuildInputs = [
            pkgs.coreutils
            pkgs.diffutils
          ];
        };

        # checks.rust-test: the hermetic Rust gate. The crate lives at
        # rust/hyphence/ but the lockfile is at the virtual-workspace root
        # (Cargo.toml/Cargo.lock), so the build source is the root manifest +
        # the rust/ tree (go/ is excluded so a Go-only change doesn't invalidate
        # the Rust check). doCheck (default) runs `cargo test`, which exercises
        # the unit tests + the RFC 0001 conformance harness (conformance.rs
        # include_str!'s rust/hyphence/testdata/rfc_vectors.txt). Zero deps, so
        # cargoLock.lockFile vendors nothing and the build is fully offline.
        rust-test = pkgs.rustPlatform.buildRustPackage {
          pname = "hyphence-rust";
          version = hyphenceVersion;
          src = pkgs.lib.fileset.toSource {
            root = ./.;
            fileset = pkgs.lib.fileset.unions [
              ./Cargo.toml
              ./Cargo.lock
              ./rust
            ];
          };
          cargoLock.lockFile = ./Cargo.lock;
        };

        # checks.vectors-equality: the Go and Rust impls each carry their own
        # copy of the normative RFC 0001 vectors (per-crate copies keep each
        # crate self-contained for a possible crates.io publish). This gate keeps
        # them byte-identical so both impls are provably tested against the same
        # vectors.
        vectors-equality = pkgs.runCommand "hyphence-vectors-equality" { } ''
          if ! diff -u \
            ${./go/hyphence/testdata/rfc_vectors.txt} \
            ${./rust/hyphence/testdata/rfc_vectors.txt}; then
            echo "rfc_vectors.txt drift between the go/ and rust/ impls — keep them byte-identical" >&2
            exit 1
          fi
          touch "$out"
        '';
      in
      {
        packages = result.packages // {
          # The store-pinned `conformist --staged --exit-zero-on-fix` hook from
          # this repo's pure-lane config; the sweatfile names it as the per-commit
          # hook. `nix build .#conformist-pre-commit` forces it.
          conformist-pre-commit = conformistEval.config.build.preCommit;

          # Likewise the merge-repair sibling: `nix build .#conformist-repair`.
          conformist-repair = conformistEval.config.build.repair;

          # The generated impure-lane config (git-state checks), consumed by
          # `just lint-worktree` to run `conformist check` against the working
          # tree where .git / host tools are available.
          conformist-impure-config = conformistImpureEval.config.build.configFile;

          # The raw conformist binary, so `just lint-worktree` can
          # `nix run .#conformist -- check --config-file <impure> --tree-root .`.
          conformist = conformist.packages.${system}.default;

          # The CLI bats lane, built by `just test-bats`.
          inherit bats-hyphence;

          # go-pkgs / go-pkgs-test: the producer outputs (filtered go/ source
          # trees) that let downstream repos bridge hyphence's Go module as a
          # flake input via goFlakeInputs, instead of the organic gomod2nix.toml
          # hash (flake-input-go_mod, RFC 0001 § Consumer interface). madder
          # bridges `github.com/amarbel-llc/hyphence/go` = go-pkgs (no subPath,
          # since the producer src is already scoped to go/).
          inherit go-pkgs go-pkgs-test;
        };

        devShells.default = result.devShells.default;

        checks = result.checks // {
          # Read-only formatting + eng-linter gate as a flake check: conformist
          # reads the generated store config and checks the read-only source tree
          # (`self`), exiting non-zero on drift. `nix flake check` runs it.
          formatting = conformistEval.config.build.check self;
          inherit rust-test vectors-equality;
        };

        # `nix fmt` runs conformist in repair mode.
        formatter = conformistEval.config.build.wrapper;
      }
    );
}
