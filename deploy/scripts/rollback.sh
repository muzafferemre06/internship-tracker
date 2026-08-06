#!/bin/sh

set -eu

SCRIPT_DIRECTORY=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=common.sh
. "$SCRIPT_DIRECTORY/common.sh"

[ "$#" -ge 2 ] && [ "$#" -le 3 ] ||
    die "usage: rollback.sh RUNTIME_ENV STATE_DIRECTORY [PUBLIC_ORIGIN]"

RUNTIME_ENV=$1
STATE_DIRECTORY=$2
PUBLIC_ORIGIN=${3:-}
CURRENT_MANIFEST=$STATE_DIRECTORY/current.env
PREVIOUS_MANIFEST=$STATE_DIRECTORY/previous.env
CURRENT_BUNDLE_DIRECTORY=$(dirname "$SCRIPT_DIRECTORY")
RELEASES_DIRECTORY=$(dirname "$CURRENT_BUNDLE_DIRECTORY")

require_file "$CURRENT_MANIFEST" "current release manifest"
require_file "$PREVIOUS_MANIFEST" "previous release manifest"
validate_release_manifest "$CURRENT_MANIFEST"
current_revision=$(manifest_value "$CURRENT_MANIFEST" DEPLOY_REVISION)
validate_deploy_bundle "$CURRENT_BUNDLE_DIRECTORY" "$current_revision"
previous_bundle=$(bundle_for_manifest "$PREVIOUS_MANIFEST" "$RELEASES_DIRECTORY")
previous_compose=$previous_bundle/compose.production.yml
"$previous_bundle/scripts/preflight.sh" \
    "$PREVIOUS_MANIFEST" "$RUNTIME_ENV" "$previous_compose" "$STATE_DIRECTORY"
acquire_deploy_lock "$STATE_DIRECTORY"

if ! compose_cmd "$RUNTIME_ENV" "$PREVIOUS_MANIFEST" "$previous_compose" pull --quiet ||
    ! compose_cmd "$RUNTIME_ENV" "$PREVIOUS_MANIFEST" "$previous_compose" \
        up --detach --remove-orphans --wait --wait-timeout 180 ||
    ! "$previous_bundle/scripts/smoke.sh" \
        "$PREVIOUS_MANIFEST" "$RUNTIME_ENV" "$previous_compose" "$PUBLIC_ORIGIN"; then
    printf '%s\n' "rollback candidate failed; restoring current release" >&2
    current_compose=$CURRENT_BUNDLE_DIRECTORY/compose.production.yml
    compose_cmd "$RUNTIME_ENV" "$CURRENT_MANIFEST" "$current_compose" pull --quiet || true
    compose_cmd "$RUNTIME_ENV" "$CURRENT_MANIFEST" "$current_compose" \
        up --detach --remove-orphans --wait --wait-timeout 180 || true
    die "rollback failed; current manifest was left unchanged"
fi

saved_current=$(mktemp "$STATE_DIRECTORY/.current.XXXXXX") || die "could not stage current manifest"
cp "$CURRENT_MANIFEST" "$saved_current"
chmod 600 "$saved_current"
copy_manifest_atomic "$PREVIOUS_MANIFEST" "$CURRENT_MANIFEST"
copy_manifest_atomic "$saved_current" "$PREVIOUS_MANIFEST"
rm -f "$saved_current"

printf '%s\n' "rollback completed; current.env and previous.env were swapped"
