#!/usr/bin/env bash
# GitHub Token Manager
#
# Provides cached GitHub App Installation Access Tokens with automatic refresh.
# This script is designed to be called frequently without performance concerns.
#
# Features:
#   - Caches token in /tmp/gh_token with expiry tracking
#   - Automatically refreshes token when less than 10 minutes remain
#   - Falls back to existing token if refresh fails
#
# Usage:
#   token=$(github-token-manager.sh)
#   # or
#   export GH_TOKEN=$(github-token-manager.sh)
#
# Dependencies:
#   - github-app-iat.sh (must be in same directory or /usr/local/bin)
#   - Credentials via env vars or /etc/github-app/ files

set -euo pipefail

TOKEN_FILE="/tmp/gh_token"
EXPIRY_FILE="/tmp/gh_token_expiry"

# Find github-app-iat.sh script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IAT_SCRIPT=""
for path in "${SCRIPT_DIR}/github-app-iat.sh" "/usr/local/bin/github-app-iat.sh"; do
    if [[ -x "$path" ]]; then
        IAT_SCRIPT="$path"
        break
    fi
done

if [[ -z "$IAT_SCRIPT" ]]; then
    # If no IAT script found, try to return cached token
    if [[ -f "$TOKEN_FILE" ]]; then
        cat "$TOKEN_FILE"
        exit 0
    fi
    echo "Error: github-app-iat.sh not found" >&2
    exit 1
fi

# Check if cached token is still valid (more than 10 minutes remaining)
if [[ -f "$TOKEN_FILE" && -f "$EXPIRY_FILE" ]]; then
    expiry=$(cat "$EXPIRY_FILE" 2>/dev/null || echo 0)
    now=$(date +%s)
    if [[ $expiry -gt $((now + 600)) ]]; then
        # Token is still valid, return it
        cat "$TOKEN_FILE"
        exit 0
    fi
fi

# Generate new token
token=$("$IAT_SCRIPT" 2>/dev/null) || true

if [[ -n "$token" ]]; then
    # Save new token and expiry (1 hour from now)
    echo "$token" > "$TOKEN_FILE"
    echo "$(($(date +%s) + 3600))" > "$EXPIRY_FILE"
    chmod 600 "$TOKEN_FILE" "$EXPIRY_FILE" 2>/dev/null || true
    echo "$token"
else
    # Token generation failed, try to return existing token as fallback
    if [[ -f "$TOKEN_FILE" ]]; then
        cat "$TOKEN_FILE"
    else
        echo "Error: Failed to generate token and no cached token available" >&2
        exit 1
    fi
fi
