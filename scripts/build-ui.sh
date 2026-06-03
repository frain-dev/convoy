#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
UIDIR="${ROOT}/api/ui/build"
DASHBOARD="${ROOT}/web/ui/dashboard"

INSTALL_DEPS="${INSTALL_DEPS:-0}"

if [[ ! -d "${DASHBOARD}" ]]; then
	echo "Dashboard directory not found: ${DASHBOARD}" >&2
	exit 1
fi

cd "${DASHBOARD}"

if [[ "${INSTALL_DEPS}" == "1" ]]; then
	echo "Installing dashboard dependencies..."
	npm install
fi

echo "Building dashboard..."
rm -rf .angular/cache dist
npm run build

mkdir -p "${UIDIR}"
rm -rf "${UIDIR:?}"/*
cp -R dist/. "${UIDIR}/"
echo -n "" > "${UIDIR}/go_test_stub.txt"

echo "Dashboard copied to ${UIDIR}"
echo "Restart the Convoy server to load the embedded UI."
