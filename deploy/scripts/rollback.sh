#!/bin/sh

set -eu

SCRIPT_DIRECTORY=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=common.sh
. "$SCRIPT_DIRECTORY/common.sh"

[ "$#" -ge 3 ] && [ "$#" -le 4 ] ||
    die "usage: rollback.sh RUNTIME_ENV COMPOSE_FILE STATE_DIRECTORY [PUBLIC_ORIGIN]"

RUNTIME_ENV=$1
COMPOSE_FILE=$2
STATE_DIRECTORY=$3
PUBLIC_ORIGIN=${4:-}
CURRENT_MANIFEST=$STATE_DIRECTORY/current.env
PREVIOUS_MANIFEST=$STATE_DIRECTORY/previous.env

require_file "$CURRENT_MANIFEST" "current release manifest"
require_file "$PREVIOUS_MANIFEST" "previous release manifest"
"$SCRIPT_DIRECTORY/preflight.sh" "$PREVIOUS_MANIFEST" "$RUNTIME_ENV" "$COMPOSE_FILE" "$STATE_DIRECTORY"
validate_release_manifest "$CURRENT_MANIFEST"
acquire_deploy_lock "$STATE_DIRECTORY"

if ! compose_cmd "$RUNTIME_ENV" "$PREVIOUS_MANIFEST" "$COMPOSE_FILE" pull --quiet ||
    ! compose_cmd "$RUNTIME_ENV" "$PREVIOUS_MANIFEST" "$COMPOSE_FILE" \
        up --detach --remove-orphans --wait --wait-timeout 180 ||
    ! "$SCRIPT_DIRECTORY/smoke.sh" "$PREVIOUS_MANIFEST" "$RUNTIME_ENV" "$COMPOSE_FILE" "$PUBLIC_ORIGIN"; then
    printf '%s\n' "rollback candidate failed; restoring current release" >&2
    compose_cmd "$RUNTIME_ENV" "$CURRENT_MANIFEST" "$COMPOSE_FILE" pull --quiet || true
    compose_cmd "$RUNTIME_ENV" "$CURRENT_MANIFEST" "$COMPOSE_FILE" \
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
