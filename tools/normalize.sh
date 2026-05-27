#!/usr/bin/env bash
# Normalize a smoketest output by redacting volatile fields so the Go and
# Java outputs are comparable line-for-line.
set -euo pipefail

if [ "$#" -ne 1 ]; then
    echo "usage: $0 <smoketest-output-file>" >&2
    exit 64
fi

sed -E \
    -e 's|^([[:space:]]+server time:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+nonce echo:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+quoteId:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+expiresAt:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+(first\|forced)[[:space:]]+bearer prefix:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+transactions:[[:space:]]+)[0-9]+$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+(services\|merchants):[[:space:]]+)[0-9]+$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+-[[:space:]]+[A-Z_]+:[[:space:]]+)[0-9]+$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+range:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+\.\.\.and[[:space:]]+)[0-9]+([[:space:]]+more)$|\1<REDACTED>\2|' \
    -e 's|PTN-[0-9]+|PTN-<REDACTED>|g' \
    -e 's|clearingDate=[^ ]+|clearingDate=<REDACTED>|g' \
    -e 's|^([[:space:]]+ptn:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+receiptNumber:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+veriCode:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+agentBalance:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+timestamp:[[:space:]]+).*$|\1<REDACTED>|' \
    -e 's|^([[:space:]]+trid:[[:space:]]+).*$|\1<REDACTED>|' \
    "$1"
