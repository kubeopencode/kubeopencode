#!/usr/bin/env bash
# GitHub CLI (gh) Wrapper with Automatic Token Injection
#
# This wrapper script automatically injects GH_TOKEN before running the gh CLI.
# It's designed to work in environments where environment variables are not
# inherited (e.g., AI agent's run_shell_command).
#
# Installation (in Dockerfile):
#   # gh is installed at /usr/bin/gh, move to gh-real
#   mv /usr/bin/gh /usr/bin/gh-real
#   ln -sf /usr/local/bin/gh-wrapper.sh /usr/bin/gh
#
# How it works:
#   1. If GH_TOKEN is already set, use the real gh directly
#   2. Otherwise, fetch token via github-token-manager.sh and inject it
#   3. Execute the real gh with all original arguments

# Find the real gh binary (not this wrapper)
REAL_GH=""
if [[ -x "/usr/bin/gh-real" ]]; then
    REAL_GH="/usr/bin/gh-real"
elif [[ -x "/usr/bin/gh" ]]; then
    REAL_GH="/usr/bin/gh"
else
    # Search PATH for gh, but skip our own directory
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    IFS=':' read -ra PATH_DIRS <<< "$PATH"
    for dir in "${PATH_DIRS[@]}"; do
        # Skip our own directory to avoid infinite loop
        [[ "$dir" == "$SCRIPT_DIR" ]] && continue
        [[ "$dir" == "/usr/local/bin" ]] && continue
        if [[ -x "$dir/gh" ]]; then
            REAL_GH="$dir/gh"
            break
        fi
    done
fi

if [[ -z "$REAL_GH" ]]; then
    echo "Error: gh CLI not found" >&2
    exit 1
fi

# If GH_TOKEN is already set, use it directly
if [[ -n "${GH_TOKEN:-}" ]]; then
    exec "$REAL_GH" "$@"
fi

# Find github-token-manager.sh
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TOKEN_MANAGER=""
for path in "${SCRIPT_DIR}/github-token-manager.sh" "/usr/local/bin/github-token-manager.sh"; do
    if [[ -x "$path" ]]; then
        TOKEN_MANAGER="$path"
        break
    fi
done

# If token manager found, inject token
if [[ -n "$TOKEN_MANAGER" ]]; then
    GH_TOKEN=$("$TOKEN_MANAGER" 2>/dev/null) || true
    if [[ -n "$GH_TOKEN" ]]; then
        export GH_TOKEN
    fi
fi

# Execute real gh with all arguments
exec "$REAL_GH" "$@"
