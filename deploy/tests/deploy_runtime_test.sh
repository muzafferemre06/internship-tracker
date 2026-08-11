#!/bin/sh

set -eu

REPOSITORY_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
TEMPORARY_DIRECTORY=$(mktemp -d)
trap 'rm -rf "$TEMPORARY_DIRECTORY"' EXIT HUP INT TERM

FAKE_BINARY_DIRECTORY=$TEMPORARY_DIRECTORY/fake-bin
RELEASES_DIRECTORY=$TEMPORARY_DIRECTORY/releases
STATE_DIRECTORY=$TEMPORARY_DIRECTORY/state
mkdir -p "$FAKE_BINARY_DIRECTORY" "$RELEASES_DIRECTORY" "$STATE_DIRECTORY" \
    "$TEMPORARY_DIRECTORY/api-secrets"

make_bundle() {
    bundle_directory=$RELEASES_DIRECTORY/$1
    mkdir -p "$bundle_directory/scripts"
    cp "$REPOSITORY_ROOT/deploy/compose.production.yml" \
        "$REPOSITORY_ROOT/deploy/nginx.production.conf" \
        "$bundle_directory/"
    cp "$REPOSITORY_ROOT"/deploy/scripts/*.sh "$bundle_directory/scripts/"
    chmod 0640 "$bundle_directory/compose.production.yml"
    chmod 0644 "$bundle_directory/nginx.production.conf"
    chmod 0750 "$bundle_directory/scripts/"*.sh
}

candidate_revision=0123456789abcdef0123456789abcdef01234567
current_revision=89abcdef0123456789abcdef0123456789abcdef
make_bundle "$candidate_revision"
make_bundle "$current_revision"

touch "$TEMPORARY_DIRECTORY/candidate.json" \
    "$TEMPORARY_DIRECTORY/sources.json"
printf '%s\n' 'fixture-tunnel-token' >"$TEMPORARY_DIRECTORY/tunnel-token"
printf '%s\n' 'fixture-web-push-private-key' \
    >"$TEMPORARY_DIRECTORY/api-secrets/web_push_private_key"
printf '%s\n' 'fixture-gemini-api-key' \
    >"$TEMPORARY_DIRECTORY/api-secrets/gemini_api_key"

cat >"$TEMPORARY_DIRECTORY/runtime.env" <<EOF
ALLOWED_ORIGIN=https://tracker.example.test
CANDIDATE_PROFILE_FILE=$TEMPORARY_DIRECTORY/candidate.json
SOURCES_FILE=$TEMPORARY_DIRECTORY/sources.json
API_SECRETS_DIRECTORY=$TEMPORARY_DIRECTORY/api-secrets
CLOUDFLARE_TUNNEL_TOKEN_FILE=$TEMPORARY_DIRECTORY/tunnel-token
DEPLOY_UID=$(id -u)
DEPLOY_GID=$(id -g)
LLM_PROVIDER=google
GEMINI_API_KEY_FILE=/run/secrets/gemini_api_key
WEB_PUSH_PUBLIC_KEY=test-public-key
WEB_PUSH_SUBJECT=mailto:operator@example.test
EOF

candidate_digest=sha256:$(printf '%064d' 0)
cat >"$TEMPORARY_DIRECTORY/release.env" <<EOF
API_IMAGE=example.invalid/api@$candidate_digest
WEB_IMAGE=example.invalid/web@$candidate_digest
CLOUDFLARED_IMAGE=example.invalid/cloudflared@$candidate_digest
DEPLOY_REVISION=$candidate_revision
EOF

current_digest=sha256:$(printf '%064d' 1)
cat >"$STATE_DIRECTORY/current.env" <<EOF
API_IMAGE=example.invalid/current-api@$current_digest
WEB_IMAGE=example.invalid/current-web@$current_digest
CLOUDFLARED_IMAGE=example.invalid/current-cloudflared@$current_digest
DEPLOY_REVISION=$current_revision
EOF
cp "$STATE_DIRECTORY/current.env" "$TEMPORARY_DIRECTORY/expected-previous.env"

FAKE_DOCKER_LOG=$TEMPORARY_DIRECTORY/fake-docker.log
FAKE_METADATA_ROOT=$TEMPORARY_DIRECTORY
FAKE_DEPLOY_UID=$(id -u)
export FAKE_DOCKER_LOG FAKE_METADATA_ROOT FAKE_DEPLOY_UID
cat >"$FAKE_BINARY_DIRECTORY/docker" <<'EOF'
#!/bin/sh

set -eu

if [ "$1" = info ]; then
    exit 0
fi
[ "$1" = compose ] || exit 91
shift

release_manifest=
while [ "$#" -gt 0 ]; do
    case "$1" in
        --project-name|--project-directory|--file)
            shift 2
            ;;
        --env-file)
            if grep -q '^API_IMAGE=' "$2"; then
                release_manifest=$2
            fi
            shift 2
            ;;
        config|pull|run|up|ps|exec|down|version)
            action=$1
            shift
            break
            ;;
        *) exit 92 ;;
    esac
done

case "$action" in
    version)
        printf '%s\n' 'Docker Compose version fixture'
        ;;
    config)
        if [ "${1:-}" = --images ]; then
            [ -n "$release_manifest" ] || exit 93
            sed -n 's/^[A-Z_]*_IMAGE=//p' "$release_manifest"
        fi
        ;;
    run)
        [ "$#" -eq 9 ] &&
            [ "$1" = --rm ] &&
            [ "$2" = --no-deps ] &&
            [ "$3" = --entrypoint ] &&
            [ "$4" = /app/backup ] &&
            [ "$5" = api ] &&
            [ "$6" = -database ] &&
            [ "$7" = /app/data/internship-tracker.db ] &&
            [ "$8" = -directory ] &&
            [ "$9" = /app/backups ] || exit 94
        printf '%s\n' "backup-run=$*" >>"$FAKE_DOCKER_LOG"
        printf '%s\n' 'snapshot=/app/backups/fixture.db'
        ;;
    ps)
        printf '%s\n' api web cloudflared
        ;;
    exec)
        [ "$#" -eq 8 ] &&
            [ "$1" = --no-TTY ] &&
            [ "$2" = web ] &&
            [ "$3" = wget ] &&
            [ "$4" = --quiet ] &&
            [ "$5" = --timeout=5 ] &&
            [ "$6" = -O ] &&
            [ "$7" = /dev/null ] &&
            [ "$8" = http://127.0.0.1:8080/ready ] || exit 97
        ;;
    pull|up|down)
        ;;
    *) exit 95 ;;
esac
EOF

cat >"$FAKE_BINARY_DIRECTORY/curl" <<'EOF'
#!/bin/sh

set -eu
printf '%s' 200
EOF

cat >"$FAKE_BINARY_DIRECTORY/stat" <<'EOF'
#!/bin/sh

set -eu

if [ "$#" -eq 4 ] && [ "$1" = -c ] && [ "$2" = '%u:%a' ] && [ "$3" = -- ]; then
    case "$4" in
        "$FAKE_METADATA_ROOT/candidate.json"|"$FAKE_METADATA_ROOT/sources.json"|\
        "$FAKE_METADATA_ROOT/api-secrets/web_push_private_key"|\
        "$FAKE_METADATA_ROOT/api-secrets/gemini_api_key")
            printf '%s\n' '100:640'
            ;;
        "$FAKE_METADATA_ROOT/api-secrets")
            printf '%s\n' '100:750'
            ;;
        "$FAKE_METADATA_ROOT/tunnel-token")
            printf '%s:%s\n' "$FAKE_DEPLOY_UID" 600
            ;;
        *) exit 96 ;;
    esac
    exit 0
fi

exec /usr/bin/stat "$@"
EOF
chmod 0755 "$FAKE_BINARY_DIRECTORY/docker" "$FAKE_BINARY_DIRECTORY/curl" \
    "$FAKE_BINARY_DIRECTORY/stat"

candidate_deploy=$RELEASES_DIRECTORY/$candidate_revision/scripts/deploy.sh
if PATH="$FAKE_BINARY_DIRECTORY:$PATH" \
    "$candidate_deploy" \
    "$TEMPORARY_DIRECTORY/release.env" \
    "$TEMPORARY_DIRECTORY/runtime.env" \
    "$STATE_DIRECTORY" \
    https://wrong.example.test \
    >"$TEMPORARY_DIRECTORY/origin-mismatch.log" 2>&1; then
    printf '%s\n' 'origin mismatch unexpectedly passed' >&2
    exit 1
fi
if ! grep -F 'public smoke origin must match ALLOWED_ORIGIN' \
    "$TEMPORARY_DIRECTORY/origin-mismatch.log" >/dev/null; then
    cat "$TEMPORARY_DIRECTORY/origin-mismatch.log" >&2
    exit 1
fi
cmp "$TEMPORARY_DIRECTORY/expected-previous.env" "$STATE_DIRECTORY/current.env"
[ ! -e "$STATE_DIRECTORY/previous.env" ]

PATH="$FAKE_BINARY_DIRECTORY:$PATH" \
    "$candidate_deploy" \
    "$TEMPORARY_DIRECTORY/release.env" \
    "$TEMPORARY_DIRECTORY/runtime.env" \
    "$STATE_DIRECTORY" \
    https://tracker.example.test \
    >"$TEMPORARY_DIRECTORY/deploy.log"

grep -Fx \
    'backup-run=--rm --no-deps --entrypoint /app/backup api -database /app/data/internship-tracker.db -directory /app/backups' \
    "$FAKE_DOCKER_LOG" >/dev/null
cmp "$TEMPORARY_DIRECTORY/release.env" "$STATE_DIRECTORY/current.env"
cmp "$TEMPORARY_DIRECTORY/expected-previous.env" "$STATE_DIRECTORY/previous.env"
if grep -E '(SECRET|TOKEN|KEY|ALLOWED_ORIGIN|API_KEY)' \
    "$STATE_DIRECTORY/current.env" "$STATE_DIRECTORY/previous.env" >/dev/null; then
    printf '%s\n' 'deployment state contains runtime or secret material' >&2
    exit 1
fi

printf '%s\n' 'deployment runtime contract passed'
