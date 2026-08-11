#!/bin/sh

set -eu

REPOSITORY_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
TEMPORARY_DIRECTORY=$(mktemp -d)
trap 'rm -rf "$TEMPORARY_DIRECTORY"' EXIT HUP INT TERM

revision=0123456789abcdef0123456789abcdef01234567
release_directory=$TEMPORARY_DIRECTORY/releases/$revision
mkdir -p "$release_directory/scripts"
cp "$REPOSITORY_ROOT/deploy/compose.production.yml" \
    "$REPOSITORY_ROOT/deploy/nginx.production.conf" \
    "$release_directory/"
cp "$REPOSITORY_ROOT"/deploy/scripts/*.sh "$release_directory/scripts/"
chmod 0640 "$release_directory/compose.production.yml"
chmod 0644 "$release_directory/nginx.production.conf"
chmod 0750 "$release_directory/scripts/"*.sh

digest=sha256:$(printf '%064d' 0)
cat >"$TEMPORARY_DIRECTORY/release.env" <<EOF
API_IMAGE=example.invalid/api@$digest
WEB_IMAGE=example.invalid/web@$digest
CLOUDFLARED_IMAGE=example.invalid/cloudflared@$digest
DEPLOY_REVISION=$revision
EOF

sh -c '
    . "$1/deploy/scripts/common.sh"
    validate_release_manifest "$2/release.env"
    validate_deploy_bundle "$2/releases/$3" "$3"
    test "$(bundle_for_manifest "$2/release.env" "$2/releases")" = "$2/releases/$3"
' sh "$REPOSITORY_ROOT" "$TEMPORARY_DIRECTORY" "$revision"

ln -s common.sh "$release_directory/scripts/linked.sh"
if sh -c '
    . "$1/deploy/scripts/common.sh"
    validate_deploy_bundle "$2/releases/$3" "$3"
' sh "$REPOSITORY_ROOT" "$TEMPORARY_DIRECTORY" "$revision" >/dev/null 2>&1; then
    printf '%s\n' "symlinked bundle entry was accepted" >&2
    exit 1
fi
rm "$release_directory/scripts/linked.sh"

touch "$release_directory/unexpected.txt"
if sh -c '
    . "$1/deploy/scripts/common.sh"
    validate_deploy_bundle "$2/releases/$3" "$3"
' sh "$REPOSITORY_ROOT" "$TEMPORARY_DIRECTORY" "$revision" >/dev/null 2>&1; then
    printf '%s\n' "unexpected bundle file was accepted" >&2
    exit 1
fi
rm "$release_directory/unexpected.txt"

if sh -c '
    . "$1/deploy/scripts/common.sh"
    validate_deploy_bundle "$2/releases/$3" ffffffffffffffffffffffffffffffffffffffff
' sh "$REPOSITORY_ROOT" "$TEMPORARY_DIRECTORY" "$revision" >/dev/null 2>&1; then
    printf '%s\n' "bundle revision mismatch was accepted" >&2
    exit 1
fi

grep -F 'ref: ${{ github.sha }}' "$REPOSITORY_ROOT/.github/workflows/publish.yml" >/dev/null
grep -F 'deploy/compose.production.yml' "$REPOSITORY_ROOT/.github/workflows/publish.yml" >/dev/null
grep -F 'deploy/nginx.production.conf' "$REPOSITORY_ROOT/.github/workflows/publish.yml" >/dev/null
grep -F "git ls-files 'deploy/scripts/*.sh'" "$REPOSITORY_ROOT/.github/workflows/publish.yml" >/dev/null
grep -F 'DEPLOY_REVISION=%s' "$REPOSITORY_ROOT/.github/workflows/publish.yml" >/dev/null
grep -F 'sha256sum "$incoming_bundle"' "$REPOSITORY_ROOT/.github/workflows/publish.yml" >/dev/null
grep -F 'inspect_output=$(docker buildx imagetools inspect "${IMAGE_NAME}:${GITHUB_SHA}")' \
    "$REPOSITORY_ROOT/.github/workflows/publish.yml" >/dev/null
if grep -F 'print $2; exit' \
    "$REPOSITORY_ROOT/.github/workflows/publish.yml" >/dev/null; then
    printf '%s\n' "immutable image digest parser exits before imagetools completes" >&2
    exit 1
fi
grep -F 'deploy/releases/<commit-sha>' "$REPOSITORY_ROOT/docs/deployment.md" >/dev/null

printf '%s\n' "exact-commit deployment bundle contract passed"
