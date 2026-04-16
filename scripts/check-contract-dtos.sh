#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTRACT_DIR="$ROOT/contracts/supervisor-ws"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gascity-contract-dtos.XXXXXX")"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

cp "$CONTRACT_DIR/package.json" "$CONTRACT_DIR/package-lock.json" "$CONTRACT_DIR/asyncapi.yaml" "$TMP_DIR/"
cp -rf "$CONTRACT_DIR/scripts" "$TMP_DIR/"

pushd "$TMP_DIR" >/dev/null
if ! npm ci --silent >"$TMP_DIR/npm-ci.stdout.log" 2>"$TMP_DIR/npm-ci.stderr.log"; then
  cat "$TMP_DIR/npm-ci.stdout.log" "$TMP_DIR/npm-ci.stderr.log"
  exit 1
fi
if ! npm run generate --silent >"$TMP_DIR/generate.stdout.log" 2>"$TMP_DIR/generate.stderr.log"; then
  cat "$TMP_DIR/generate.stdout.log" "$TMP_DIR/generate.stderr.log"
  exit 1
fi
popd >/dev/null

if ! diff -ru "$CONTRACT_DIR/generated" "$TMP_DIR/generated"; then
  echo
  echo "Error: supervisor-ws generated DTOs are stale."
  echo "Run:"
  echo "  npm --prefix contracts/supervisor-ws ci"
  echo "  npm --prefix contracts/supervisor-ws run generate"
  exit 1
fi
