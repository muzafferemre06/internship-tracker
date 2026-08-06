#!/bin/sh

set -eu

SCRIPT_DIRECTORY=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=common.sh
. "$SCRIPT_DIRECTORY/common.sh"

[ "$#" -ge 4 ] && [ "$#" -le 5 ] ||
    die "usage: deploy.sh RELEASE_ENV RUNTIME_ENV COMPOSE_FILE STATE_DIRECTORY [PUBLIC_ORIGIN]"

RELEASE_MANIFEST=$1
RUNTIME_ENV=$2
COMPOSE_FILE=$3
STATE_DIRECTORY=$4
PUBLIC_ORIGIN=${5:-}
CURRENT_MANIFEST=$STATE_DIRECTORY/current.env
PREVIOUS_MANIFEST=$STATE_DIRECTORY/previous.env

"$SCRIPT_DIRECTORY/preflight.sh" "$RELEASE_MANIFEST" "$RUNTIME_ENV" "$COMPOSE_FILE" "$STATE_DIRECTORY"
acquire_deploy_lock "$STATE_DIRECTORY"

deploy_candidate() {
    compose_cmd "$RUNTIME_ENV" "$RELEASE_MANIFEST" "$COMPOSE_FILE" pull --quiet &&
        compose_cmd "$RUNTIME_ENV" "$RELEASE_MANIFEST" "$COMPOSE_FILE" \
            up --detach --remove-orphans --wait --wait-timeout 180 &&
        "$SCRIPT_DIRECTORY/smoke.sh" "$RELEASE_MANIFEST" "$RUNTIME_ENV" "$COMPOSE_FILE" "$PUBLIC_ORIGIN"
}

snapshot_current_database() {
    if [ ! -f "$CURRENT_MANIFEST" ]; then
        printf '%s\n' "no current release manifest; skipping snapshot on first deployment"
        return
    fi
    validate_release_manifest "$CURRENT_MANIFEST"

    printf '%s\n' "creating a consistent pre-deployment SQLite snapshot"
    compose_cmd "$RUNTIME_ENV" "$CURRENT_MANIFEST" "$COMPOSE_FILE" \
        run --rm --no-deps api /app/backup \
        -database /app/data/internship-tracker.db \
        -directory /app/backups
}

restore_current() {
    if [ ! -f "$CURRENT_MANIFEST" ]; then
        printf '%s\n' "first deployment failed; stopping the incomplete release" >&2
        compose_cmd "$RUNTIME_ENV" "$RELEASE_MANIFEST" "$COMPOSE_FILE" down --remove-orphans || true
        return 1
    fi

    printf '%s\n' "candidate failed; restoring current image manifest" >&2
    validate_release_manifest "$CURRENT_MANIFEST"
    compose_cmd "$RUNTIME_ENV" "$CURRENT_MANIFEST" "$COMPOSE_FILE" pull --quiet &&
        compose_cmd "$RUNTIME_ENV" "$CURRENT_MANIFEST" "$COMPOSE_FILE" \
            up --detach --remove-orphans --wait --wait-timeout 180 &&
        "$SCRIPT_DIRECTORY/smoke.sh" "$CURRENT_MANIFEST" "$RUNTIME_ENV" "$COMPOSE_FILE" "$PUBLIC_ORIGIN"
}

snapshot_current_database

if ! deploy_candidate; then
    if restore_current; then
        die "candidate deployment failed and the previous release was restored"
    fi
    die "candidate deployment failed and automatic recovery also failed"
fi

if [ -f "$CURRENT_MANIFEST" ]; then
    copy_manifest_atomic "$CURRENT_MANIFEST" "$PREVIOUS_MANIFEST"
fi
copy_manifest_atomic "$RELEASE_MANIFEST" "$CURRENT_MANIFEST"

printf '%s\n' "deployment completed and current.env was updated"
