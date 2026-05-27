#!/usr/bin/env bash
# Normalize a smoketest output by redacting volatile fields so the Go and
# Java outputs are comparable line-for-line.
set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "usage: $0 <smoketest-output-file>" >&2
    exit 64
fi

sed -E \
    -e 's|^(\s+server time:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+nonce echo:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+quoteId:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+expiresAt:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+(first\|forced)\s+bearer prefix:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+transactions:\s+)[0-9]+$|\1<REDACTED>|' \
    -e 's|^(\s+(services\|merchants):\s+)[0-9]+$|\1<REDACTED>|' \
    -e 's|^(\s+-\s+[A-Z_]+:\s+)[0-9]+$|\1<REDACTED>|' \
    -e 's|^(\s+range:\s+).*$|\1<REDACTED>|' \
    -e 's|^(\s+\.\.\.and\s+)[0-9]+(\s+more)$|\1<REDACTED>\2|' \
    -e 's|PTN-[0-9]+|PTN-<REDACTED>|g' \
    "$1"
