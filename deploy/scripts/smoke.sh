#!/bin/sh

set -eu

SCRIPT_DIRECTORY=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=common.sh
. "$SCRIPT_DIRECTORY/common.sh"

[ "$#" -ge 3 ] && [ "$#" -le 4 ] ||
    die "usage: smoke.sh RELEASE_ENV RUNTIME_ENV COMPOSE_FILE [PUBLIC_ORIGIN]"

RELEASE_MANIFEST=$1
RUNTIME_ENV=$2
COMPOSE_FILE=$3
PUBLIC_ORIGIN=${4:-}

validate_release_manifest "$RELEASE_MANIFEST"

running_services=$(compose_cmd "$RUNTIME_ENV" "$RELEASE_MANIFEST" "$COMPOSE_FILE" ps --status running --services)
for service_name in api web cloudflared; do
    printf '%s\n' "$running_services" | grep -Fx "$service_name" >/dev/null ||
        die "$service_name is not running"
done

compose_cmd "$RUNTIME_ENV" "$RELEASE_MANIFEST" "$COMPOSE_FILE" \
    exec --no-TTY web wget --quiet --timeout=5 --spider http://127.0.0.1:8080/ready ||
    die "origin readiness smoke check failed"

if [ -n "$PUBLIC_ORIGIN" ]; then
    printf '%s\n' "$PUBLIC_ORIGIN" | grep -Eq '^https://[^/?#]+/?$' ||
        die "public smoke origin must be a path-free HTTPS URL"
    PUBLIC_ORIGIN=${PUBLIC_ORIGIN%/}

    if [ -n "${CF_ACCESS_CLIENT_ID_FILE:-}" ] || [ -n "${CF_ACCESS_CLIENT_SECRET_FILE:-}" ]; then
        [ -n "${CF_ACCESS_CLIENT_ID_FILE:-}" ] && [ -n "${CF_ACCESS_CLIENT_SECRET_FILE:-}" ] ||
            die "both Cloudflare Access credential files must be configured"
        require_file "$CF_ACCESS_CLIENT_ID_FILE" "Cloudflare Access client ID"
        require_file "$CF_ACCESS_CLIENT_SECRET_FILE" "Cloudflare Access client secret"
        access_client_id=$(tr -d '\r\n' < "$CF_ACCESS_CLIENT_ID_FILE")
        access_client_secret=$(tr -d '\r\n' < "$CF_ACCESS_CLIENT_SECRET_FILE")
        printf '%s\n' "$access_client_id" | grep -Eq '^[A-Za-z0-9._-]+$' || die "invalid Access client ID file"
        printf '%s\n' "$access_client_secret" | grep -Eq '^[A-Za-z0-9._-]+$' || die "invalid Access client secret file"
        public_status=$(printf 'header = "CF-Access-Client-Id: %s"\nheader = "CF-Access-Client-Secret: %s"\n' \
            "$access_client_id" "$access_client_secret" |
            curl --config - --silent --show-error --max-time 20 \
                --output /dev/null --write-out '%{http_code}' "$PUBLIC_ORIGIN/ready") ||
            die "public readiness smoke check failed"
        unset access_client_id access_client_secret
    else
        public_status=$(curl --silent --show-error --max-time 20 \
            --output /dev/null --write-out '%{http_code}' "$PUBLIC_ORIGIN/ready") ||
            die "public readiness smoke check failed"
    fi
    [ "$public_status" = 200 ] || die "public readiness returned HTTP $public_status instead of 200"
fi

printf '%s\n' "smoke checks passed"
