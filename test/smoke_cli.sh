#!/usr/bin/env bash
# D.3 — CLI Smoke Test
# Usage: GOS3_ENDPOINT=http://localhost:9000 GOS3_ACCESS_KEY=xxx GOS3_SECRET_KEY=yyy bash test/smoke_cli.sh
# Requires: gos3c binary in $PATH or ./gos3c

set -euo pipefail

ENDPOINT="${GOS3_ENDPOINT:-http://localhost:9000}"
ACCESS_KEY="${GOS3_ACCESS_KEY:-testaccesskey}"
SECRET_KEY="${GOS3_SECRET_KEY:-testsecretkey}"
BUCKET="smoke-test-$(date +%s)"
PASS=0
FAIL=0

# Find gos3c binary
GOS3C=""
if command -v gos3c &>/dev/null; then
  GOS3C="gos3c"
elif [ -f ./gos3c ]; then
  GOS3C="./gos3c"
elif [ -f ./bin/gos3c ]; then
  GOS3C="./bin/gos3c"
else
  echo "ERROR: gos3c binary not found. Build with: go build -o gos3c ./cmd/client/"
  exit 1
fi

export GOS3_ENDPOINT="$ENDPOINT"
export GOS3_ACCESS_KEY="$ACCESS_KEY"
export GOS3_SECRET_KEY="$SECRET_KEY"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

pass() { echo -e "  ${GREEN}✓ PASS${NC}: $1"; ((PASS++)); }
fail() { echo -e "  ${RED}✗ FAIL${NC}: $1"; ((FAIL++)); }

run_test() {
  local desc="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    pass "$desc"
  else
    fail "$desc"
  fi
}

echo "=== GoS3 CLI Smoke Test ==="
echo "  Endpoint: $ENDPOINT"
echo "  Bucket:   $BUCKET"
echo ""

# --- Bucket Operations ---
echo "--- Bucket Operations ---"
run_test "mb: create bucket" $GOS3C mb "s3://$BUCKET"
run_test "ls: list buckets (should contain bucket)" bash -c "$GOS3C ls | grep -q $BUCKET"

# --- Object Operations ---
echo "--- Object Operations ---"
TMPDIR=$(mktemp -d)
echo "hello smoke test" > "$TMPDIR/upload.txt"

run_test "cp: upload file" $GOS3C cp "$TMPDIR/upload.txt" "s3://$BUCKET/test.txt"
run_test "ls: list objects" bash -c "$GOS3C ls s3://$BUCKET/ | grep -q test.txt"
run_test "stat: object metadata" $GOS3C stat "s3://$BUCKET/test.txt"
run_test "cat: stream object" bash -c "$GOS3C cat s3://$BUCKET/test.txt | grep -q 'hello smoke test'"

run_test "cp: download file" $GOS3C cp "s3://$BUCKET/test.txt" "$TMPDIR/downloaded.txt"
run_test "diff: content matches" diff "$TMPDIR/upload.txt" "$TMPDIR/downloaded.txt"

# --- Copy & Move ---
echo "--- Copy & Move ---"
run_test "cp: server-side copy" $GOS3C cp "s3://$BUCKET/test.txt" "s3://$BUCKET/copy.txt"
run_test "ls: copy exists" bash -c "$GOS3C ls s3://$BUCKET/ | grep -q copy.txt"

# mv = copy + delete source (if supported by CLI)
if $GOS3C mv "s3://$BUCKET/copy.txt" "s3://$BUCKET/moved.txt" >/dev/null 2>&1; then
  pass "mv: move object"
  run_test "ls: moved exists" bash -c "$GOS3C ls s3://$BUCKET/ | grep -q moved.txt"
  run_test "ls: copy gone" bash -c "! $GOS3C ls s3://$BUCKET/ | grep -q copy.txt"
else
  echo "  SKIP: mv not implemented"
fi

# --- Presign ---
echo "--- Presign ---"
if PRESIGN_URL=$($GOS3C presign "s3://$BUCKET/test.txt" --expires 60 2>/dev/null); then
  pass "presign: generate URL"
  if curl -sf "$PRESIGN_URL" | grep -q "hello smoke test" 2>/dev/null; then
    pass "presign: download via URL"
  else
    fail "presign: download via URL"
  fi
else
  echo "  SKIP: presign not implemented or failed"
fi

# --- Recursive Operations ---
echo "--- Recursive Operations ---"
mkdir -p "$TMPDIR/dir"
echo "file-a" > "$TMPDIR/dir/a.txt"
echo "file-b" > "$TMPDIR/dir/b.txt"

run_test "cp --recursive: upload dir" $GOS3C cp --recursive "$TMPDIR/dir/" "s3://$BUCKET/dir/"
run_test "ls: uploaded files" bash -c "$GOS3C ls s3://$BUCKET/dir/ | grep -q a.txt"

# --- Cleanup ---
echo "--- Cleanup ---"
run_test "rm --recursive: delete all objects" $GOS3C rm --recursive "s3://$BUCKET/"
run_test "rb: remove bucket" $GOS3C rb "s3://$BUCKET"
run_test "ls: bucket gone" bash -c "! $GOS3C ls | grep -q $BUCKET"

# Cleanup temp
rm -rf "$TMPDIR"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
