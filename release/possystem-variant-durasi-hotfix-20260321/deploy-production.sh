#!/usr/bin/env bash
set -euo pipefail

POSSYSTEM_APP_DIR="${POSSYSTEM_APP_DIR:-/var/www/applications/v2/possystem}"
POSSYSTEM_SERVICE="${POSSYSTEM_SERVICE:-possystem}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
STAMP="$(date +%Y%m%d%H%M%S)"

require_root() {
    if [[ "${EUID}" -ne 0 ]]; then
        echo "Please run this script with sudo."
        echo "Example: sudo bash ${SCRIPT_DIR}/deploy-production.sh"
        exit 1
    fi
}

require_file() {
    local file_path="$1"
    if [[ ! -f "${file_path}" ]]; then
        echo "Required file not found: ${file_path}"
        exit 1
    fi
}

backup_if_exists() {
    local path="$1"
    if [[ -e "${path}" ]]; then
        local backup_path="${path}.bak.${STAMP}"
        echo "Backing up ${path} -> ${backup_path}"
        cp -R "${path}" "${backup_path}"
    fi
}

resolve_binary_name() {
    local machine
    machine="$(uname -m)"

    case "${machine}" in
        x86_64|amd64)
            printf '%s\n' "possystem-linux-amd64"
            ;;
        aarch64|arm64)
            printf '%s\n' "possystem-linux-arm64"
            ;;
        *)
            echo "Unsupported server architecture: ${machine}" >&2
            exit 1
            ;;
    esac
}

install_possystem_binary() {
    local binary_name
    binary_name="$(resolve_binary_name)"

    local source_binary="${SCRIPT_DIR}/${binary_name}"
    local target_binary="${POSSYSTEM_APP_DIR}/possystem"

    require_file "${source_binary}"
    mkdir -p "${POSSYSTEM_APP_DIR}"
    backup_if_exists "${target_binary}"

    echo "Installing ${binary_name} to ${target_binary}"
    install -m 755 "${source_binary}" "${target_binary}"
}

restart_service_if_exists() {
    if systemctl cat "${POSSYSTEM_SERVICE}" >/dev/null 2>&1; then
        echo "Restarting ${POSSYSTEM_SERVICE}.service"
        systemctl restart "${POSSYSTEM_SERVICE}"
    else
        echo "Service ${POSSYSTEM_SERVICE}.service not found. Restart it manually if needed."
    fi
}

print_summary() {
    echo
    echo "Deploy complete."
    echo "Suggested checks:"
    echo "  systemctl status ${POSSYSTEM_SERVICE} --no-pager -l"
    echo "  ls -lh ${POSSYSTEM_APP_DIR}/possystem"
    echo "  curl -k -I https://pos.zenter.co.id"
    echo
    echo "Functional check:"
    echo "  Open adminfitness > POS Service Variants and save an item with numeric durasi."
}

main() {
    require_root
    install_possystem_binary
    restart_service_if_exists
    print_summary
}

main "$@"
