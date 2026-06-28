# Build/check half of hyphence's go/ subtree. hyphence is a LIBRARY (no
# binaries this phase), so the only build output is `checks.go-test` — a
# hermetic `go test -tags test ./...` gate — plus the Go devShell. The
# conformist format/lint lanes are wired in flake.nix (they need
# conformist.lib + `self`); the derived hooks/binaries are passed in here
# for the devShell only.
{
  nixpkgs,
  nixpkgs-master,
  # conformist (treefmt successor): the BARE binary on the devShell PATH
  # (the wrapper is the flake `formatter` output). Defaulted null so a
  # direct `import ./go/default.nix` without flake context still works.
  conformist ? null,
  # doppelgang provides `lint`, the flake.lock dedup gate. Defaulted null
  # so non-flake callers don't need it.
  doppelgang ? null,
  # The module-generated, toolchain-hermetic per-commit hook wrapper, on
  # the devShell PATH as `conformist-pre-commit` (the sweatfile
  # [hooks].pre-commit command). Defaulted null for non-flake callers.
  conformistPreCommit ? null,
  system,
  # Flake-input bridge table (see ./gomod.nix). Defaulted {} so non-flake
  # callers degrade to organic gomod2nix.toml resolution.
  goFlakeInputs ? { },
  version ? "dev",
}:
let
  # The fork's default.nix shim auto-applies overlays.default (the gomod2nix
  # overlay carrying goFlakeInputs support), so no explicit overlay here.
  pkgs = import nixpkgs { inherit system; };
  pkgs-master = import nixpkgs-master { inherit system; };

  # checks.go-test: the hermetic CI gate. CROWN JEWEL: the RFC conformance
  # suite (go/hyphence/rfc_conformance_test.go) is behind `//go:build test`,
  # so the `-tags test` flag is MANDATORY — without it the conformance
  # vectors silently don't run.
  #
  # hyphence is a binary-less library module, so the stock buildGoApplication
  # build/install (which expects a `main` to install) is replaced with a plain
  # `go build ./...` + the tagged test suite. configurePhase (gomod2nix dep
  # vendoring + the dewey bridge) is left intact, so module resolution is
  # unchanged.
  go-test = pkgs.buildGoApplication {
    pname = "hyphence-go-test";
    inherit version goFlakeInputs;
    src = ./.;
    pwd = ./.;
    modules = ./gomod2nix.toml;
    go = pkgs-master.go_1_26;
    GOTOOLCHAIN = "local";
    subPackages = [ "hyphence" ];

    buildPhase = ''
      runHook preBuild
      go build ./...
      runHook postBuild
    '';

    doCheck = true;
    checkPhase = ''
      runHook preCheck
      go test -tags test -p $NIX_BUILD_CORES ./...
      runHook postCheck
    '';

    installPhase = ''
      runHook preInstall
      mkdir -p "$out"
      runHook postInstall
    '';
  };
in
{
  packages = { };

  checks = {
    inherit go-test;
  };

  devShells.default = pkgs-master.mkShell {
    packages = [
      (pkgs.mkGoEnv {
        pwd = ./.;
        inherit goFlakeInputs;
      })
      # gomod2nix CLI for `just build-gomod2nix` / `just update-go`. Sourced
      # from igloo (matching the gomod2nix the conformist drift linter runs)
      # so the regenerated gomod2nix.toml byte-matches what the linter expects.
      pkgs.gomod2nix
    ]
    ++ pkgs-master.lib.optionals (doppelgang != null) [
      doppelgang.packages.${system}.default
    ]
    # The BARE conformist binary plus the formatter/linter tools the generated
    # config drives that aren't in the pkgs-master block below.
    ++ pkgs-master.lib.optionals (conformist != null) [
      conformist.packages.${system}.default
      pkgs.nixfmt
      pkgs-master.shellcheck
    ]
    # The per-commit hook wrapper, on PATH as `conformist-pre-commit` for the
    # sweatfile [hooks].pre-commit command.
    ++ pkgs-master.lib.optionals (conformistPreCommit != null) [
      conformistPreCommit
    ]
    ++ (with pkgs-master; [
      gofumpt
      gopls
      gotools
      just
      shfmt
    ]);

    GOTOOLCHAIN = "local";
  };
}
