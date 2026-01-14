#!/usr/bin/env bash
# Git Credential Helper for GitHub App Authentication
#
# This script implements the git credential helper protocol to provide
# automatic authentication for git operations against github.com.
#
# Installation:
#   git config --global credential.helper /usr/local/bin/git-credential-github-app.sh
#   # or
#   git config --system credential.helper /usr/local/bin/git-credential-github-app.sh
#
# How it works:
#   When git needs credentials for github.com, it calls this script with "get".
#   The script fetches a token via github-token-manager.sh and returns it
#   in the git credential format.
#
# Dependencies:
#   - github-token-manager.sh (must be in same directory or /usr/local/bin)

set -euo pipefail

# Find github-token-manager.sh script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOKEN_MANAGER=""
for path in "${SCRIPT_DIR}/github-token-manager.sh" "/usr/local/bin/github-token-manager.sh"; do
    if [[ -x "$path" ]]; then
        TOKEN_MANAGER="$path"
        break
    fi
done

case "${1:-}" in
    get)
        # Read credential request from stdin
        is_github=0
        protocol=""
        host=""

        while IFS= read -r line; do
            # Empty line signals end of input
            [[ -z "$line" ]] && break

            case "$line" in
                protocol=*)
                    protocol="${line#protocol=}"
                    ;;
                host=*)
                    host="${line#host=}"
                    [[ "$host" == "github.com" ]] && is_github=1
                    ;;
            esac
        done

        # Only provide credentials for github.com
        if [[ "$is_github" == 1 ]]; then
            if [[ -z "$TOKEN_MANAGER" ]]; then
                echo "Error: github-token-manager.sh not found" >&2
                exit 1
            fi

            token=$("$TOKEN_MANAGER" 2>/dev/null) || true

            if [[ -n "$token" ]]; then
                echo "protocol=https"
                echo "host=github.com"
                echo "username=x-access-token"
                echo "password=$token"
            fi
        fi
        ;;

    store|erase)
        # These operations are not needed for GitHub App tokens
        # Token lifecycle is managed by github-token-manager.sh
        ;;

    *)
        # Unknown operation, ignore
        ;;
esac
