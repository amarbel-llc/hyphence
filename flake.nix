{
  description = "hyphence — the hyphen-fence document format (text-based metadata + body serialization)";

  inputs = {
    # amarbel-llc/nixpkgs fork. Pre-bundles buildGoApplication / mkGoEnv (the
    # gomod2nix overlay, with goFlakeInputs support per nixpkgs RFC 0001) into
    # the base pkgs set, so this flake's build closure matches madder's.
    igloo = {
      url = "github:amarbel-llc/igloo";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.treefmt-nix.follows = "bats/treefmt-nix";
    };

    # SHA-pinned upstream anchor (Hydra-blessed, cache.nixos.org-served).
    # Sourced as pkgs-master in go/default.nix for go_1_26 + shell tools.
    nixpkgs-master.url = "github:NixOS/nixpkgs/567a49d1913ce81ac6e9582e3553dd90a955875f";

    utils = {
      url = "https://flakehub.com/f/numtide/flake-utils/0.1.102";
      inputs.systems.follows = "igloo/systems";
    };

    # bats: amarbel-llc/bats provides lib.batsLane for future CLI/integration
    # lanes. No bats suite yet (library-only), so it's an input + follows anchor
    # for the shared treefmt-nix node; not consumed in an output yet.
    bats = {
      url = "github:amarbel-llc/bats";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.igloo.follows = "igloo";
      inputs.utils.follows = "utils";
    };

    # purse-first: dewey's home. Sourced via go/gomod.nix's goFlakeInputs to
    # bridge github.com/amarbel-llc/purse-first/libs/dewey, so a dewey bump is
    # a flake.lock-only edit (RFC 0001 § Consumer interface).
    purse-first = {
      url = "github:amarbel-llc/purse-first";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.igloo.follows = "igloo";
      inputs.utils.follows = "utils";
      inputs.conformist.follows = "conformist";
    };

    # conformist: the linter/formatter multiplexer (treefmt successor). Config
    # is Nix-generated from ./conformist.nix + presets.eng via evalModule.
    conformist = {
      url = "github:amarbel-llc/conformist";
      inputs.igloo.follows = "igloo";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
    };

    # doppelgang provides `lint`, the flake.lock dedup gate. On the devShell
    # PATH; the follows wiring above keeps one node per shared input.
    doppelgang = {
      url = "github:amarbel-llc/doppelgang";
      inputs.igloo.follows = "igloo";
      inputs.nixpkgs-master.follows = "nixpkgs-master";
      inputs.utils.follows = "utils";
      inputs.treefmt-nix.follows = "bats/treefmt-nix";
    };
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

        # Pure-consumer goFlakeInputs map (bridges dewey). See go/gomod.nix.
        goFlakeInputs = import ./go/gomod.nix {
          inherit purse-first system;
        };

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
          imports = [ conformist.lib.presets.eng-impure ];
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
        };
      in
      {
        packages = result.packages // {
          # The store-pinned `conformist --staged --exit-zero-on-fix` hook from
          # this repo's pure-lane config; the sweatfile names it as the per-commit
          # hook. `nix build .#conformist-pre-commit` forces it.
          conformist-pre-commit = conformistEval.config.build.preCommit;

          # The generated impure-lane config (git-state checks), consumed by
          # `just lint-worktree` to run `conformist check` against the working
          # tree where .git / host tools are available.
          conformist-impure-config = conformistImpureEval.config.build.configFile;

          # The raw conformist binary, so `just lint-worktree` can
          # `nix run .#conformist -- check --config-file <impure> --tree-root .`.
          conformist = conformist.packages.${system}.default;
        };

        devShells.default = result.devShells.default;

        checks = result.checks // {
          # Read-only formatting + eng-linter gate as a flake check: conformist
          # reads the generated store config and checks the read-only source tree
          # (`self`), exiting non-zero on drift. `nix flake check` runs it.
          formatting = conformistEval.config.build.check self;
        };

        # `nix fmt` runs conformist in repair mode.
        formatter = conformistEval.config.build.wrapper;
      }
    );
}
