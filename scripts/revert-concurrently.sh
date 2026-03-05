#!/bin/bash
# Revert CONCURRENTLY removal - run after sqlc generate is done
# Usage: ./scripts/revert-concurrently.sh

cd "$(dirname "$0")/.."
git checkout -- sql/
echo "Reverted CONCURRENTLY in migrations (restored from git)"
