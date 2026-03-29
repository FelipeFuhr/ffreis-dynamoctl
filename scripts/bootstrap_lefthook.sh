#!/usr/bin/env bash
# Bootstrap script: installs lefthook and registers git hooks.
set -euo pipefail

echo "==> Checking required tools..."
bash "$(dirname "$0")/hooks/check_required_tools.sh"

echo "==> Installing lefthook git hooks..."
lefthook install

echo "==> Done. Git hooks are active."
