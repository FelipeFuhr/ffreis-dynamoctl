#!/usr/bin/env bash
# Verify that required development tools are available.
set -euo pipefail

REQUIRED_TOOLS=(go gofmt golangci-lint lefthook)
missing=()

for tool in "${REQUIRED_TOOLS[@]}"; do
  if ! command -v "$tool" &>/dev/null; then
    missing+=("$tool")
  fi
done

if [[ ${#missing[@]} -gt 0 ]]; then
  echo "Missing required tools: ${missing[*]}" >&2
  echo "Install them before continuing." >&2
  exit 1
fi

echo "All required tools are present."
