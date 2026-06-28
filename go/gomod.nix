# Nix side of go.mod for hyphence — the mixed producer+consumer half of the
# flake-input-go_mod protocol (amarbel-llc/nixpkgs RFC 0001):
#
#   - producer: mkGoPkgs publishes go-pkgs / go-pkgs-test so downstream repos
#     (madder) can bridge hyphence's Go module as a flake input instead of the
#     organic gomod2nix.toml hash. The src is scoped to the go/ subdir by the
#     caller, so downstream bridges with NO subPath.
#
#   - consumer: goFlakeInputs bridges github.com/amarbel-llc/purse-first/libs/
#     dewey onto purse-first's go-pkgs output, so a dewey bump is a flake.lock-
#     only edit. purse-first's go-pkgs is the whole workspace, so we slice into
#     the dewey module subdir with subPath.
#
# Modules NOT bridged here (mvdan.cc/sh, the golang.org/x deps) continue to
# resolve through gomod2nix.toml.
{
  pkgs,
  src,
  purse-first,
  system,
}:
let
  goFlakeInputs = {
    "github.com/amarbel-llc/purse-first/libs/dewey" = {
      src = purse-first.packages.${system}.go-pkgs;
      subPath = "libs/dewey";
    };
  };
in
{
  # mkGoPkgs filters the go/ source tree into go-pkgs (build superset) and
  # go-pkgs-test (test superset). goFlakeInputs is threaded so the outputs carry
  # passthru.goFlakeInputs, letting downstream consumers inherit hyphence's dewey
  # bridge at depth-1 (RFC 0001 § Multi-producer closures).
  goPkgs = pkgs.mkGoPkgs { inherit src goFlakeInputs; };
  inherit goFlakeInputs;
}
