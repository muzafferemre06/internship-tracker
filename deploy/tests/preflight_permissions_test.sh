#!/bin/sh

set -eu

REPOSITORY_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
TEMPORARY_DIRECTORY=$(mktemp -d)
trap 'rm -rf "$TEMPORARY_DIRECTORY"' EXIT HUP INT TERM

fake_bin=$TEMPORARY_DIRECTORY/bin
state_directory=$TEMPORARY_DIRECTORY/state
secrets_directory=$TEMPORARY_DIRECTORY/api-secrets
mkdir -p "$fake_bin" "$state_directory" "$secrets_directory"

printf '%s\n' '{}' >"$TEMPORARY_DIRECTORY/candidate.json"
printf '%s\n' '{}' >"$TEMPORARY_DIRECTORY/sources.json"
printf '%s\n' 'tunnel-token' >"$TEMPORARY_DIRECTORY/tunnel-token"
printf '%s\n' 'web-push-private-key' >"$secrets_directory/web_push_private_key"
printf '%s\n' 'gemini-api-key' >"$secrets_directory/gemini_api_key"

cat >"$fake_bin/id" <<'EOF'
#!/bin/sh
case "${1:-}" in
    -u|-g) printf '%s\n' 1000 ;;
    *) exit 1 ;;
esac
EOF

cat >"$fake_bin/stat" <<'EOF'
#!/bin/sh
for argument in "$@"; do
    checked_path=$argument
done
case "$(basename "$checked_path")" in
    candidate.json) metadata=${CANDIDATE_METADATA:-100:640} ;;
    sources.json) metadata=${SOURCES_METADATA:-100:640} ;;
    api-secrets) metadata=${API_DIRECTORY_METADATA:-100:750} ;;
    web_push_private_key) metadata=${WEB_PUSH_METADATA:-100:640} ;;
    gemini_api_key) metadata=${GEMINI_METADATA:-100:640} ;;
    tunnel-token) metadata=${TUNNEL_METADATA:-1000:600} ;;
    *) printf '%s\n' "unexpected stat path: $checked_path" >&2; exit 1 ;;
esac
printf '%s\n' "$metadata"
EOF

cat >"$fake_bin/docker" <<EOF
#!/bin/sh
case "\$*" in
    info|"compose version") exit 0 ;;
    *"config --images")
        printf '%s\n' \\
            'example.invalid/api@sha256:$(printf '%064d' 0)' \\
            'example.invalid/web@sha256:$(printf '%064d' 0)' \\
            'example.invalid/cloudflared@sha256:$(printf '%064d' 0)'
        ;;
    *"config --quiet") exit 0 ;;
    *) printf '%s\n' "unexpected docker invocation: \$*" >&2; exit 1 ;;
esac
EOF
chmod 0755 "$fake_bin/id" "$fake_bin/stat" "$fake_bin/docker"

cat >"$TEMPORARY_DIRECTORY/runtime.env" <<EOF
ALLOWED_ORIGIN=https://tracker.example.test
CANDIDATE_PROFILE_FILE=$TEMPORARY_DIRECTORY/candidate.json
SOURCES_FILE=$TEMPORARY_DIRECTORY/sources.json
API_SECRETS_DIRECTORY=$secrets_directory
CLOUDFLARE_TUNNEL_TOKEN_FILE=$TEMPORARY_DIRECTORY/tunnel-token
DEPLOY_UID=1000
DEPLOY_GID=1000
LLM_PROVIDER=google
GEMINI_API_KEY_FILE=/run/secrets/gemini_api_key
WEB_PUSH_PUBLIC_KEY=test-public-key
WEB_PUSH_SUBJECT=mailto:operator@example.test
EOF

digest=sha256:$(printf '%064d' 0)
cat >"$TEMPORARY_DIRECTORY/release.env" <<EOF
API_IMAGE=example.invalid/api@$digest
WEB_IMAGE=example.invalid/web@$digest
CLOUDFLARED_IMAGE=example.invalid/cloudflared@$digest
DEPLOY_REVISION=0123456789abcdef0123456789abcdef01234567
EOF

run_preflight() {
    PATH="$fake_bin:$PATH" "$REPOSITORY_ROOT/deploy/scripts/preflight.sh" \
        "$TEMPORARY_DIRECTORY/release.env" \
        "$TEMPORARY_DIRECTORY/runtime.env" \
        "$REPOSITORY_ROOT/deploy/compose.production.yml" \
        "$state_directory"
}

expect_permission_failure() {
    variable_name=$1
    metadata=$2
    expected_message=$3
    output_file=$TEMPORARY_DIRECTORY/failure-output
    if env "$variable_name=$metadata" PATH="$fake_bin:$PATH" \
        "$REPOSITORY_ROOT/deploy/scripts/preflight.sh" \
        "$TEMPORARY_DIRECTORY/release.env" \
        "$TEMPORARY_DIRECTORY/runtime.env" \
        "$REPOSITORY_ROOT/deploy/compose.production.yml" \
        "$state_directory" >"$output_file" 2>&1; then
        printf '%s\n' "$variable_name=$metadata was accepted" >&2
        exit 1
    fi
    grep -F "$expected_message" "$output_file" >/dev/null || {
        printf '%s\n' "missing preflight error: $expected_message" >&2
        cat "$output_file" >&2
        exit 1
    }
}

run_preflight | grep -F 'preflight passed' >/dev/null
expect_permission_failure CANDIDATE_METADATA 1000:640 \
    'CANDIDATE_PROFILE_FILE must be owned by UID 100 with mode 0640'
expect_permission_failure SOURCES_METADATA 100:600 \
    'SOURCES_FILE must be owned by UID 100 with mode 0640'
expect_permission_failure API_DIRECTORY_METADATA 1000:750 \
    'API_SECRETS_DIRECTORY must be owned by UID 100 with mode 0750'
expect_permission_failure WEB_PUSH_METADATA 100:600 \
    'Web Push private key must be owned by UID 100 with mode 0640'
expect_permission_failure GEMINI_METADATA 1000:640 \
    'Gemini API key must be owned by UID 100 with mode 0640'
expect_permission_failure TUNNEL_METADATA 100:600 \
    'Cloudflare Tunnel token must be owned by UID 1000 with mode 0600'

printf '%s\n' 'production preflight permission contract passed'
