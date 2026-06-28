# Nix side of go.mod for hyphence — the pure-consumer half of the
# flake-input-go_mod protocol (amarbel-llc/nixpkgs RFC 0001).
#
# hyphence's Go module imports only dewey subpackages (errors, interfaces,
# ohio, format, files, catgut, values, flags) + stdlib. Bridging dewey
# through purse-first's `go-pkgs` output means a dewey bump is a flake.lock-
# only edit — no `go get` + `gomod2nix generate` lockstep. Every other
# module in go.sum continues to resolve through gomod2nix.toml.
#
# purse-first's go-pkgs is the whole workspace (multiple Go modules under
# one repo root), so we slice into the dewey module subdir with subPath.
{
  purse-first,
  system,
}:
{
  "github.com/amarbel-llc/purse-first/libs/dewey" = {
    src = purse-first.packages.${system}.go-pkgs;
    subPath = "libs/dewey";
  };
}
