#!/usr/bin/env bash
# Fail if total test coverage is below COVERAGE_THRESHOLD (default 80%).
set -euo pipefail
IFS=$'\n\t'

THRESHOLD="${COVERAGE_THRESHOLD:-80}"

go test -timeout 60s -coverprofile=coverage.out ./... 1>&2

total=$(go tool cover -func=coverage.out | awk '/^total:/ { gsub(/%/, "", $3); print $3 }')
echo "Total coverage: ${total}%" >&2

awk "BEGIN {
  if ($total < $THRESHOLD) {
    print \"Coverage ${total}% is below threshold ${THRESHOLD}%\" > \"/dev/stderr\"
    exit 1
  }
}"
