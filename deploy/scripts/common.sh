#!/bin/sh

set -eu

PROJECT_NAME=internship-tracker

die() {
    printf '%s\n' "error: $*" >&2
    exit 1
}

require_file() {
    [ -f "$1" ] && [ -r "$1" ] || die "$2 is not a readable file: $1"
}

require_absolute_path() {
    case "$1" in
        /*) ;;
        *) die "$2 must be an absolute path: $1" ;;
    esac
}

env_value() {
    env_file=$1
    env_key=$2
    awk -v wanted="$env_key" '
        /^[[:space:]]*(#|$)/ { next }
        index($0, wanted "=") == 1 {
            value = substr($0, length(wanted) + 2)
            sub(/\r$/, "", value)
            if ((substr(value, 1, 1) == "\"" && substr(value, length(value), 1) == "\"") ||
                (substr(value, 1, 1) == "\047" && substr(value, length(value), 1) == "\047")) {
                value = substr(value, 2, length(value) - 2)
            }
            print value
            found++
        }
        END { if (found > 1) exit 2 }
    ' "$env_file"
}

manifest_value() {
    manifest_value_result=$(env_value "$1" "$2") || die "duplicate $2 entry in $1"
    [ -n "$manifest_value_result" ] || die "$2 is missing from $1"
    printf '%s\n' "$manifest_value_result"
}

validate_digest_reference() {
    image_name=$1
    image_reference=$2
    printf '%s\n' "$image_reference" | grep -Eq '^[^[:space:]@]+@sha256:[0-9a-f]{64}$' ||
        die "$image_name must be pinned as repository@sha256:<64 lowercase hex characters>"
}

validate_release_manifest() {
    release_manifest=$1
    require_file "$release_manifest" "release manifest"

    awk -F= '
        /^[[:space:]]*(#|$)/ { next }
        $0 !~ /^(API_IMAGE|WEB_IMAGE|CLOUDFLARED_IMAGE|DEPLOY_REVISION)=[^=]+$/ { exit 1 }
        seen[$1]++ > 0 { exit 1 }
        END {
            if (seen["API_IMAGE"] != 1 || seen["WEB_IMAGE"] != 1 ||
                seen["CLOUDFLARED_IMAGE"] != 1 || seen["DEPLOY_REVISION"] != 1) exit 1
        }
    ' "$release_manifest" ||
        die "release manifest must contain exactly one API_IMAGE, WEB_IMAGE, CLOUDFLARED_IMAGE and DEPLOY_REVISION"

    for image_name in API_IMAGE WEB_IMAGE CLOUDFLARED_IMAGE; do
        image_reference=$(manifest_value "$release_manifest" "$image_name")
        validate_digest_reference "$image_name" "$image_reference"
    done

    deploy_revision=$(manifest_value "$release_manifest" DEPLOY_REVISION)
    printf '%s\n' "$deploy_revision" | grep -Eq '^[0-9a-f]{40}$' ||
        die "DEPLOY_REVISION must be a full lowercase Git commit SHA"
}

validate_deploy_bundle() {
    bundle_directory=$1
    expected_revision=$2

    require_absolute_path "$bundle_directory" "deploy bundle directory"
    [ -d "$bundle_directory" ] && [ ! -L "$bundle_directory" ] ||
        die "deploy bundle is not a real directory: $bundle_directory"
    [ "$(basename "$bundle_directory")" = "$expected_revision" ] ||
        die "deploy bundle directory does not match revision $expected_revision"

    for relative_path in \
        compose.production.yml \
        nginx.production.conf \
        scripts/common.sh \
        scripts/deploy.sh \
        scripts/preflight.sh \
        scripts/rollback.sh \
        scripts/smoke.sh; do
        bundle_file=$bundle_directory/$relative_path
        [ -f "$bundle_file" ] && [ ! -L "$bundle_file" ] && [ -r "$bundle_file" ] ||
            die "deploy bundle entry must be a readable regular file: $bundle_file"
        case "$relative_path" in
            scripts/*.sh)
                [ -x "$bundle_file" ] && [ "$(stat -c '%a' "$bundle_file")" = 750 ] ||
                    die "deploy bundle script mode must be 0750: $bundle_file"
                ;;
            *)
                [ "$(stat -c '%a' "$bundle_file")" = 640 ] ||
                    die "deploy bundle config mode must be 0640: $bundle_file"
                ;;
        esac
    done

    unexpected_entry=$(find "$bundle_directory" -mindepth 1 \
        ! -type d ! -type f -print -quit)
    [ -z "$unexpected_entry" ] || die "deploy bundle contains a symlink or special entry: $unexpected_entry"

    unexpected_directory=$(find "$bundle_directory" -mindepth 1 -type d \
        ! -path "$bundle_directory/scripts" -print -quit)
    [ -z "$unexpected_directory" ] ||
        die "deploy bundle contains an unexpected directory: $unexpected_directory"

    unexpected_file=$(find "$bundle_directory" -type f \
        ! -path "$bundle_directory/compose.production.yml" \
        ! -path "$bundle_directory/nginx.production.conf" \
        ! -path "$bundle_directory/scripts/common.sh" \
        ! -path "$bundle_directory/scripts/deploy.sh" \
        ! -path "$bundle_directory/scripts/preflight.sh" \
        ! -path "$bundle_directory/scripts/rollback.sh" \
        ! -path "$bundle_directory/scripts/smoke.sh" -print -quit)
    [ -z "$unexpected_file" ] || die "deploy bundle contains an unexpected file: $unexpected_file"
}

bundle_for_manifest() {
    release_manifest=$1
    releases_directory=$2
    deploy_revision=$(manifest_value "$release_manifest" DEPLOY_REVISION)
    bundle_directory=$releases_directory/$deploy_revision
    validate_deploy_bundle "$bundle_directory" "$deploy_revision"
    printf '%s\n' "$bundle_directory"
}

compose_cmd() {
    runtime_env=$1
    release_manifest=$2
    compose_file=$3
    shift 3

    # Shell values otherwise override --env-file values in Docker Compose.
    env \
        -u API_IMAGE -u WEB_IMAGE -u CLOUDFLARED_IMAGE \
        -u ALLOWED_ORIGIN -u LLM_PROVIDER -u LLM_MODEL \
        -u LLM_THINKING_LEVEL -u LLM_INPUT_COST_PER_MILLION_USD \
        -u LLM_OUTPUT_COST_PER_MILLION_USD -u OPENROUTER_API_KEY_FILE \
        -u GEMINI_API_KEY_FILE -u SCAN_SCHEDULE -u SCAN_TIMEZONE \
        -u BACKUP_TIME -u BACKUP_TIMEZONE -u BACKUP_RETENTION \
        -u WEB_PUSH_PUBLIC_KEY -u WEB_PUSH_SUBJECT \
        -u CANDIDATE_PROFILE_FILE -u SOURCES_FILE -u API_SECRETS_DIRECTORY \
        -u NGINX_CONFIG_FILE -u CLOUDFLARE_TUNNEL_TOKEN_FILE \
        -u DEPLOY_UID -u DEPLOY_GID \
        docker compose --project-name "$PROJECT_NAME" \
        --project-directory "$(dirname "$compose_file")" \
        --env-file "$runtime_env" --env-file "$release_manifest" \
        --file "$compose_file" "$@"
}

copy_manifest_atomic() {
    source_manifest=$1
    destination_manifest=$2
    destination_directory=$(dirname "$destination_manifest")
    temporary_manifest=$(mktemp "$destination_directory/.manifest.XXXXXX") ||
        die "could not create a temporary manifest in $destination_directory"
    if ! cp "$source_manifest" "$temporary_manifest" ||
        ! chmod 600 "$temporary_manifest" ||
        ! mv -f "$temporary_manifest" "$destination_manifest"; then
        rm -f "$temporary_manifest"
        die "could not update manifest $destination_manifest"
    fi
}

acquire_deploy_lock() {
    state_directory=$1
    lock_directory="$state_directory/deploy.lock"
    if ! mkdir "$lock_directory" 2>/dev/null; then
        die "another deployment or rollback holds $lock_directory"
    fi
    trap 'rmdir "$lock_directory" 2>/dev/null || true' EXIT HUP INT TERM
}
