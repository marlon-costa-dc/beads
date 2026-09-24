#!/usr/bin/env bash
# Shared hermetic environment setup for broad local/CI test wrappers.

if [[ -n "${BEADS_CI_TEST_ENV_SH_LOADED:-}" ]]; then
    return 0
fi
BEADS_CI_TEST_ENV_SH_LOADED=1

# The invoking user's persistent beads cache directory. Callers resolve it
# before beads_test_env_enter replaces HOME with the disposable sandbox home.
beads_test_cache_dir() {
    printf '%s\n' "${XDG_CACHE_HOME:-$HOME/.cache}/beads"
}

beads_test_env_enter() {
    if [[ "${BEADS_TEST_ENV_DISABLE:-0}" == "1" ]]; then
        return 0
    fi
    if [[ "${BEADS_TEST_ENV_ACTIVE:-0}" == "1" ]]; then
        return 0
    fi

    # The disposable root lives under the invoking user's cache, never under
    # system /tmp: broad runs create Dolt roots, sockets and Go build scratch
    # that must stay on a capacity-managed volume. The root may therefore have
    # a real ~/.beads among its ancestors; BEADS_TEST_DISCOVERY_CEILING below
    # stops every ancestor walk at the root so that ledger is never found.
    local tmp_base root
    tmp_base="${BEADS_TEST_TMP_BASE:-$(beads_test_cache_dir)/test-tmp}"
    mkdir -p "$tmp_base"
    root="$(mktemp -d "$tmp_base/beads-test-env-XXXXXX")"
    export BEADS_TEST_ENV_ROOT="$root"
    export BEADS_TEST_ENV_ACTIVE=1
    export BEADS_TEST_DISCOVERY_CEILING="$root"

    if [[ -z "${GOCACHE:-}" ]]; then
        local go_cache
        go_cache="$(go env GOCACHE 2>/dev/null || true)"
        if [[ -n "$go_cache" ]]; then
            export GOCACHE="$go_cache"
        fi
    fi
    # Docker resolves its endpoint from the current context in ~/.docker,
    # which the sandbox HOME hides: testcontainers would then probe the
    # default socket and every container-backed Dolt test would skip. Pin the
    # endpoint the caller's Docker CLI selects before HOME is replaced.
    if [[ -z "${DOCKER_HOST:-}" ]] && command -v docker >/dev/null 2>&1; then
        local docker_host
        docker_host="$(docker context inspect --format '{{.Endpoints.docker.Host}}')"
        export DOCKER_HOST="$docker_host"
    fi
    if [[ -z "${GOMODCACHE:-}" ]]; then
        local go_mod_cache
        go_mod_cache="$(go env GOMODCACHE 2>/dev/null || true)"
        if [[ -n "$go_mod_cache" ]]; then
            export GOMODCACHE="$go_mod_cache"
        fi
    fi

    mkdir -p "$root/home" "$root/xdg-config" "$root/dolt-root" "$root/tmp" "$root/go-tmp"
    : >"$root/gitconfig"

    export HOME="$root/home"
    # t.TempDir() and go's build scratch land inside the root, below the
    # discovery ceiling, instead of in the caller's TMPDIR tree.
    export TMPDIR="$root/tmp"
    export GOTMPDIR="$root/go-tmp"
    export USERPROFILE="$root/home"
    export XDG_CONFIG_HOME="$root/xdg-config"
    export DOLT_ROOT_PATH="$root/dolt-root"
    export GIT_CONFIG_NOSYSTEM=1
    export GIT_CONFIG_GLOBAL="$root/gitconfig"
    export BEADS_TEST_IGNORE_REPO_CONFIG=1
    if [[ "${BEADS_TEST_ENV_RUN_DOLT:-0}" != "1" ]]; then
        beads_test_env_add_skip "dolt"
    fi

    unset BEADS_DIR
    unset BEADS_DB
    unset BD_DB
    unset BD_JSON
    unset BD_NO_DB
    unset BD_NO_DAEMON
    unset BD_ACTOR
    unset BEADS_ACTOR
    unset GT_ROOT
    unset BEADS_DOLT_SHARED_SERVER
    unset BEADS_DOLT_SERVER_MODE
    unset BEADS_DOLT_AUTO_START
    unset BEADS_DOLT_SERVER_HOST
    unset BEADS_DOLT_SERVER_PORT
    unset BEADS_DOLT_PORT
    unset BEADS_DOLT_SERVER_DATABASE
    unset BEADS_DOLT_SERVER_SOCKET
    unset BEADS_DOLT_PASSWORD

    if command -v dolt >/dev/null 2>&1; then
        dolt config --global --add user.name "beads-test" >/dev/null 2>&1 || true
        dolt config --global --add user.email "test@beads.local" >/dev/null 2>&1 || true
    fi

    trap beads_test_env_cleanup EXIT
}

beads_test_env_add_skip() {
    local service="$1"
    local current=",${BEADS_TEST_SKIP:-},"
    if [[ "$current" != *",$service,"* ]]; then
        if [[ -n "${BEADS_TEST_SKIP:-}" ]]; then
            export BEADS_TEST_SKIP="${BEADS_TEST_SKIP},${service}"
        else
            export BEADS_TEST_SKIP="$service"
        fi
    fi
}

beads_test_env_cleanup() {
    if [[ "${BEADS_TEST_ENV_KEEP:-0}" == "1" ]]; then
        return 0
    fi
    if [[ -n "${BEADS_TEST_ENV_ROOT:-}" ]]; then
        rm -rf "$BEADS_TEST_ENV_ROOT"
        unset BEADS_TEST_ENV_ROOT
        unset BEADS_TEST_DISCOVERY_CEILING
    fi
}
