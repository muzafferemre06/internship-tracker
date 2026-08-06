#!/bin/sh

set -eu

SCRIPT_DIRECTORY=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=common.sh
. "$SCRIPT_DIRECTORY/common.sh"

[ "$#" -eq 4 ] || die "usage: preflight.sh RELEASE_ENV RUNTIME_ENV COMPOSE_FILE STATE_DIRECTORY"

RELEASE_MANIFEST=$1
RUNTIME_ENV=$2
COMPOSE_FILE=$3
STATE_DIRECTORY=$4

[ "$(id -u)" -ne 0 ] || die "deployment must run as a dedicated non-root operator"
require_file "$RUNTIME_ENV" "runtime environment"
require_file "$COMPOSE_FILE" "production Compose file"
require_absolute_path "$RELEASE_MANIFEST" "release manifest"
require_absolute_path "$RUNTIME_ENV" "runtime environment"
require_absolute_path "$COMPOSE_FILE" "production Compose file"
require_absolute_path "$STATE_DIRECTORY" "state directory"
[ -d "$STATE_DIRECTORY" ] && [ -w "$STATE_DIRECTORY" ] ||
    die "state directory must already exist and be writable: $STATE_DIRECTORY"

for command_name in awk basename curl dirname docker env find grep id mktemp sed stat; do
    command -v "$command_name" >/dev/null 2>&1 || die "required command is missing: $command_name"
done
docker info >/dev/null 2>&1 || die "the non-root operator cannot reach the Docker daemon"
docker compose version >/dev/null 2>&1 || die "Docker Compose v2 is required"

deploy_uid=$(env_value "$RUNTIME_ENV" DEPLOY_UID) || die "duplicate DEPLOY_UID in $RUNTIME_ENV"
deploy_gid=$(env_value "$RUNTIME_ENV" DEPLOY_GID) || die "duplicate DEPLOY_GID in $RUNTIME_ENV"
printf '%s\n' "$deploy_uid" | grep -Eq '^[0-9]+$' || die "DEPLOY_UID must be numeric"
printf '%s\n' "$deploy_gid" | grep -Eq '^[0-9]+$' || die "DEPLOY_GID must be numeric"
[ "$deploy_uid" -ne 0 ] && [ "$deploy_gid" -ne 0 ] || die "deployment UID and GID must be non-root"
[ "$deploy_uid" -eq "$(id -u)" ] || die "DEPLOY_UID must match the deployment operator UID"
[ "$deploy_gid" -eq "$(id -g)" ] || die "DEPLOY_GID must match the deployment operator primary GID"

require_owner_mode() {
    checked_path=$1
    checked_label=$2
    expected_owner=$3
    expected_mode=$4

    actual_metadata=$(stat -c '%u:%a' -- "$checked_path") ||
        die "could not inspect ownership and mode for $checked_label: $checked_path"
    [ "$actual_metadata" = "$expected_owner:$expected_mode" ] ||
        die "$checked_label must be owned by UID $expected_owner with mode 0$expected_mode: $checked_path (got $actual_metadata)"
}

validate_release_manifest "$RELEASE_MANIFEST"

if grep -Eq '^(API_IMAGE|WEB_IMAGE|CLOUDFLARED_IMAGE|OPENROUTER_API_KEY|GEMINI_API_KEY|WEB_PUSH_PRIVATE_KEY|TUNNEL_TOKEN)=' "$RUNTIME_ENV"; then
    die "runtime environment must not contain image references or direct secret values"
fi

allowed_origin=$(env_value "$RUNTIME_ENV" ALLOWED_ORIGIN) || die "duplicate ALLOWED_ORIGIN in $RUNTIME_ENV"
printf '%s\n' "$allowed_origin" | grep -Eq '^https://[^/?#]+$' ||
    die "ALLOWED_ORIGIN must be a path-free HTTPS origin"

for path_key in CANDIDATE_PROFILE_FILE SOURCES_FILE CLOUDFLARE_TUNNEL_TOKEN_FILE; do
    host_path=$(env_value "$RUNTIME_ENV" "$path_key") || die "duplicate $path_key in $RUNTIME_ENV"
    [ -n "$host_path" ] || die "$path_key is missing from $RUNTIME_ENV"
    require_absolute_path "$host_path" "$path_key"
    require_file "$host_path" "$path_key"
done

candidate_profile_file=$(env_value "$RUNTIME_ENV" CANDIDATE_PROFILE_FILE)
sources_file=$(env_value "$RUNTIME_ENV" SOURCES_FILE)
tunnel_token_file=$(env_value "$RUNTIME_ENV" CLOUDFLARE_TUNNEL_TOKEN_FILE)
require_owner_mode "$candidate_profile_file" "CANDIDATE_PROFILE_FILE" 100 640
require_owner_mode "$sources_file" "SOURCES_FILE" 100 640
require_owner_mode "$tunnel_token_file" "Cloudflare Tunnel token" "$deploy_uid" 600

secrets_directory=$(env_value "$RUNTIME_ENV" API_SECRETS_DIRECTORY) ||
    die "duplicate API_SECRETS_DIRECTORY in $RUNTIME_ENV"
[ -n "$secrets_directory" ] || die "API_SECRETS_DIRECTORY is missing from $RUNTIME_ENV"
require_absolute_path "$secrets_directory" "API_SECRETS_DIRECTORY"
[ -d "$secrets_directory" ] && [ -r "$secrets_directory" ] ||
    die "API_SECRETS_DIRECTORY is not readable: $secrets_directory"
require_owner_mode "$secrets_directory" "API_SECRETS_DIRECTORY" 100 750
require_file "$secrets_directory/web_push_private_key" "Web Push private key"
[ -s "$secrets_directory/web_push_private_key" ] || die "Web Push private key is empty"
require_owner_mode "$secrets_directory/web_push_private_key" "Web Push private key" 100 640

provider=$(env_value "$RUNTIME_ENV" LLM_PROVIDER) || die "duplicate LLM_PROVIDER in $RUNTIME_ENV"
provider=${provider:-deterministic}
case "$provider" in
    deterministic) ;;
    openrouter)
        provider_path=$(env_value "$RUNTIME_ENV" OPENROUTER_API_KEY_FILE) ||
            die "duplicate OPENROUTER_API_KEY_FILE in $RUNTIME_ENV"
        [ "$provider_path" = /run/secrets/openrouter_api_key ] ||
            die "OPENROUTER_API_KEY_FILE must be /run/secrets/openrouter_api_key"
        require_file "$secrets_directory/openrouter_api_key" "OpenRouter API key"
        [ -s "$secrets_directory/openrouter_api_key" ] || die "OpenRouter API key is empty"
        require_owner_mode "$secrets_directory/openrouter_api_key" "OpenRouter API key" 100 640
        ;;
    google|gemini)
        provider_path=$(env_value "$RUNTIME_ENV" GEMINI_API_KEY_FILE) ||
            die "duplicate GEMINI_API_KEY_FILE in $RUNTIME_ENV"
        [ "$provider_path" = /run/secrets/gemini_api_key ] ||
            die "GEMINI_API_KEY_FILE must be /run/secrets/gemini_api_key"
        require_file "$secrets_directory/gemini_api_key" "Gemini API key"
        [ -s "$secrets_directory/gemini_api_key" ] || die "Gemini API key is empty"
        require_owner_mode "$secrets_directory/gemini_api_key" "Gemini API key" 100 640
        ;;
    *) die "unsupported LLM_PROVIDER in runtime environment: $provider" ;;
esac

[ -s "$tunnel_token_file" ] || die "Cloudflare Tunnel token is empty"

compose_cmd "$RUNTIME_ENV" "$RELEASE_MANIFEST" "$COMPOSE_FILE" config --quiet

configured_images=$(compose_cmd "$RUNTIME_ENV" "$RELEASE_MANIFEST" "$COMPOSE_FILE" config --images)
for image_name in API_IMAGE WEB_IMAGE CLOUDFLARED_IMAGE; do
    image_reference=$(manifest_value "$RELEASE_MANIFEST" "$image_name")
    printf '%s\n' "$configured_images" | grep -Fx "$image_reference" >/dev/null ||
        die "$image_name was not preserved in rendered Compose configuration"
done

printf '%s\n' "preflight passed"
