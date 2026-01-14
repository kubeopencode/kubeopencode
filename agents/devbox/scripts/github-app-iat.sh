#!/usr/bin/env bash
# GitHub App Installation Access Token (IAT) Generator
#
# This script generates an Installation Access Token from GitHub App credentials.
# The IAT can be used as a Bearer token for GitHub API requests.
#
# Credential sources (in priority order):
#   1. Environment variables:
#      - GH_APP_ID           - GitHub App ID
#      - GH_APP_INSTALLATION_ID  - Installation ID for the target org/repo
#      - GH_APP_PRIVATE_KEY  - PEM-formatted private key (full content)
#
#   2. File paths (KubeOpenCode default):
#      - /etc/github-app/client_id       (or GH_APP_ID file)
#      - /etc/github-app/installation_id (or GH_APP_INSTALLATION_ID file)
#      - /etc/github-app/private_key     (or GH_APP_PRIVATE_KEY file)
#
# Note: We use GH_APP_* prefix instead of GITHUB_* because GitHub Actions
#       reserves the GITHUB_* namespace and doesn't allow user secrets with that prefix.
#
# Usage:
#   # With environment variables
#   export GH_APP_ID="123456"
#   export GH_APP_PRIVATE_KEY="$(cat private-key.pem)"
#   export GH_APP_INSTALLATION_ID="789012"
#   ./github-app-iat.sh
#
#   # With file paths (KubeOpenCode)
#   # Credentials mounted to /etc/github-app/ by controller
#   ./github-app-iat.sh
#
# Output: The IAT token (e.g., ghs_xxxxxxxxxxxx) on stdout
# Errors are written to stderr

set -euo pipefail

# Default credential directory (KubeOpenCode convention)
GITHUB_APP_DIR="${GITHUB_APP_DIR:-/etc/github-app}"

# Load credentials from files if environment variables are not set
load_credentials_from_files() {
    # App ID
    if [[ -z "${GH_APP_ID:-}" ]]; then
        local id_file="${GITHUB_APP_DIR}/client_id"
        if [[ -f "$id_file" ]]; then
            GH_APP_ID=$(cat "$id_file")
            export GH_APP_ID
        fi
    fi

    # Installation ID
    if [[ -z "${GH_APP_INSTALLATION_ID:-}" ]]; then
        local install_file="${GITHUB_APP_DIR}/installation_id"
        if [[ -f "$install_file" ]]; then
            GH_APP_INSTALLATION_ID=$(cat "$install_file")
            export GH_APP_INSTALLATION_ID
        fi
    fi

    # Private Key
    if [[ -z "${GH_APP_PRIVATE_KEY:-}" ]]; then
        local key_file="${GITHUB_APP_DIR}/private_key"
        if [[ -f "$key_file" ]]; then
            GH_APP_PRIVATE_KEY=$(cat "$key_file")
            export GH_APP_PRIVATE_KEY
        fi
    fi
}

# Base64URL encoding (required for JWT)
# Standard base64 with: + -> -, / -> _, trailing = removed
b64url_encode() {
    openssl enc -base64 -A | tr '+/' '-_' | tr -d '='
}

# Generate JWT from GitHub App credentials
# Args: app_id, private_key
# Returns: JWT string (valid for ~9 minutes)
generate_jwt() {
    local app_id="$1"
    local private_key="$2"
    local now
    now=$(date +%s)

    # JWT Header
    local header='{"alg":"RS256","typ":"JWT"}'

    # JWT Payload
    # - iat: issued at (60 seconds in the past to account for clock drift)
    # - exp: expiration (9 minutes from now, max is 10 minutes)
    # - iss: issuer (GitHub App ID)
    local payload
    payload=$(printf '{"iat":%d,"exp":%d,"iss":"%s"}' \
        "$((now - 60))" \
        "$((now + 540))" \
        "$app_id")

    # Encode header and payload
    local header_b64
    local payload_b64
    header_b64=$(printf '%s' "$header" | b64url_encode)
    payload_b64=$(printf '%s' "$payload" | b64url_encode)

    local unsigned_token="${header_b64}.${payload_b64}"

    # Sign with RS256 (RSA-SHA256)
    local signature
    signature=$(printf '%s' "$unsigned_token" | \
        openssl dgst -binary -sha256 -sign <(printf '%s' "$private_key") | \
        b64url_encode)

    printf '%s.%s' "$unsigned_token" "$signature"
}

# Exchange JWT for Installation Access Token
# Args: jwt, installation_id
# Returns: IAT token on success, exits with error on failure
get_installation_access_token() {
    local jwt="$1"
    local installation_id="$2"

    local response
    local http_code
    local body

    # Make API request
    response=$(curl -s -w "\n%{http_code}" -X POST \
        "https://api.github.com/app/installations/${installation_id}/access_tokens" \
        -H "Authorization: Bearer $jwt" \
        -H "Accept: application/vnd.github+json" \
        -H "X-GitHub-Api-Version: 2022-11-28")

    # Extract HTTP code (last line) and body (everything else)
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [[ "$http_code" != "201" ]]; then
        echo "Error: Failed to get IAT (HTTP $http_code)" >&2
        local error_message
        error_message=$(echo "$body" | jq -r '.message // "Unknown error"' 2>/dev/null || echo "Unknown error")
        echo "  Message: $error_message" >&2
        return 1
    fi

    # Extract token from response
    local token
    token=$(echo "$body" | jq -r '.token' 2>/dev/null)

    if [[ -z "$token" || "$token" == "null" ]]; then
        echo "Error: Failed to parse IAT from response" >&2
        return 1
    fi

    printf '%s' "$token"
}

# Main function
main() {
    # Try to load credentials from files first
    load_credentials_from_files

    # Validate required environment variables
    if [[ -z "${GH_APP_ID:-}" ]]; then
        echo "Error: GH_APP_ID not set (env var or ${GITHUB_APP_DIR}/client_id)" >&2
        return 1
    fi

    if [[ -z "${GH_APP_INSTALLATION_ID:-}" ]]; then
        echo "Error: GH_APP_INSTALLATION_ID not set (env var or ${GITHUB_APP_DIR}/installation_id)" >&2
        return 1
    fi

    if [[ -z "${GH_APP_PRIVATE_KEY:-}" ]]; then
        echo "Error: GH_APP_PRIVATE_KEY not set (env var or ${GITHUB_APP_DIR}/private_key)" >&2
        return 1
    fi

    # Validate private key format
    if [[ ! "$GH_APP_PRIVATE_KEY" =~ ^-----BEGIN.*PRIVATE\ KEY----- ]]; then
        echo "Error: GH_APP_PRIVATE_KEY does not appear to be a valid PEM key" >&2
        return 1
    fi

    # Check for required tools
    if ! command -v openssl &> /dev/null; then
        echo "Error: openssl is required but not installed" >&2
        return 1
    fi

    if ! command -v jq &> /dev/null; then
        echo "Error: jq is required but not installed" >&2
        return 1
    fi

    # Generate JWT
    local jwt
    jwt=$(generate_jwt "$GH_APP_ID" "$GH_APP_PRIVATE_KEY")

    if [[ -z "$jwt" ]]; then
        echo "Error: Failed to generate JWT" >&2
        return 1
    fi

    # Exchange JWT for IAT
    local iat
    iat=$(get_installation_access_token "$jwt" "$GH_APP_INSTALLATION_ID")

    if [[ -z "$iat" ]]; then
        return 1
    fi

    # Output the IAT token
    printf '%s' "$iat"
}

# Run main function
main "$@"
