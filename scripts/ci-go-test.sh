#!/usr/bin/env bash
# Shared wrapper for GitHub Actions: optional -race and -v from env.
# GO_TEST_RACE=1 (default) enables -race. GO_TEST_VERBOSE=1 (default) enables -v.
set -euo pipefail
R=""
[ "${GO_TEST_RACE:-1}" = "1" ] && R="-race"
VF=""
[ "${GO_TEST_VERBOSE:-1}" = "1" ] && VF="-v"
exec go test $R $VF "$@"
