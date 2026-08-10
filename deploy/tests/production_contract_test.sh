#!/bin/sh

set -eu

REPOSITORY_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
TEMPORARY_DIRECTORY=$(mktemp -d)
trap 'rm -rf "$TEMPORARY_DIRECTORY"' EXIT HUP INT TERM

digest=sha256:$(printf '%064d' 0)
touch "$TEMPORARY_DIRECTORY/candidate.json" \
    "$TEMPORARY_DIRECTORY/sources.json" \
    "$TEMPORARY_DIRECTORY/tunnel-token"
mkdir "$TEMPORARY_DIRECTORY/api-secrets"
touch "$TEMPORARY_DIRECTORY/api-secrets/web_push_private_key"

grep -F 'name: internship-tracker' "$REPOSITORY_ROOT/deploy/compose.production.yml" >/dev/null
grep -F 'source: tracker_data' "$REPOSITORY_ROOT/deploy/compose.production.yml" >/dev/null
grep -F 'target: /app/data' "$REPOSITORY_ROOT/deploy/compose.production.yml" >/dev/null
grep -F '/out/dbinspect ./cmd/dbinspect' "$REPOSITORY_ROOT/deploy/backend.Dockerfile" >/dev/null
grep -F 'COPY --from=builder /out/dbinspect /app/dbinspect' "$REPOSITORY_ROOT/deploy/backend.Dockerfile" >/dev/null

cat >"$TEMPORARY_DIRECTORY/runtime.env" <<EOF
ALLOWED_ORIGIN=https://tracker.example.test
CANDIDATE_PROFILE_FILE=$TEMPORARY_DIRECTORY/candidate.json
SOURCES_FILE=$TEMPORARY_DIRECTORY/sources.json
NGINX_CONFIG_FILE=$REPOSITORY_ROOT/deploy/nginx.production.conf
API_SECRETS_DIRECTORY=$TEMPORARY_DIRECTORY/api-secrets
CLOUDFLARE_TUNNEL_TOKEN_FILE=$TEMPORARY_DIRECTORY/tunnel-token
DEPLOY_UID=1000
DEPLOY_GID=1000
LLM_PROVIDER=google
OPENROUTER_API_KEY_FILE=/run/secrets/openrouter_api_key
GEMINI_API_KEY_FILE=/run/secrets/gemini_api_key
WEB_PUSH_PUBLIC_KEY=test-public-key
WEB_PUSH_SUBJECT=mailto:operator@example.test
EOF

cat >"$TEMPORARY_DIRECTORY/release.env" <<EOF
API_IMAGE=example.invalid/api@$digest
WEB_IMAGE=example.invalid/web@$digest
CLOUDFLARED_IMAGE=example.invalid/cloudflared@$digest
EOF

if [ "${SKIP_COMPOSE_RENDER:-0}" = 1 ]; then
    printf '%s\n' 'skipping Compose render by explicit local request'
else
    docker compose \
        --env-file "$TEMPORARY_DIRECTORY/runtime.env" \
        --env-file "$TEMPORARY_DIRECTORY/release.env" \
        --file "$REPOSITORY_ROOT/deploy/compose.production.yml" \
        config >"$TEMPORARY_DIRECTORY/rendered.yml"

    grep -F 'GEMINI_API_KEY_FILE: /run/secrets/gemini_api_key' \
        "$TEMPORARY_DIRECTORY/rendered.yml" >/dev/null
    grep -F 'OPENROUTER_API_KEY_FILE: /run/secrets/openrouter_api_key' \
        "$TEMPORARY_DIRECTORY/rendered.yml" >/dev/null
fi
"$REPOSITORY_ROOT/deploy/tests/deploy_workflow_test.sh"
"$REPOSITORY_ROOT/deploy/tests/deploy_runtime_test.sh"

printf '%s\n' "production deployment contract passed"
