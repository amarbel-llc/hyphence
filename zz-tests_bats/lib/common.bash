#!/usr/bin/env bash
# Shared bats helpers for the hyphence CLI suite. Minimal by design: the suite
# only needs bats-assert/bats-support and a runner for the built binary.

bats_load_library bats-support
bats_load_library bats-assert

# run_hyphence runs the built hyphence binary under bats `run` so $status and
# $output are captured for the assert_* helpers. HYPHENCE_BIN is exported by the
# nix bats lane (pointing at the built binary); it falls back to `hyphence` on
# PATH for ad-hoc runs inside the devShell.
run_hyphence() {
  run "${HYPHENCE_BIN:-hyphence}" "$@"
}
