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

install_possystem_binary() {
    local source_binary="${SCRIPT_DIR}/possystem-linux-amd64"
    local target_binary="${POSSYSTEM_APP_DIR}/possystem"

    require_file "${source_binary}"
    mkdir -p "${POSSYSTEM_APP_DIR}"
    backup_if_exists "${target_binary}"

    echo "Installing possystem binary to ${target_binary}"
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
    echo "  curl -k -I https://pos.zenter.co.id"
    echo
    echo "Functional check:"
    echo "  Open adminfitness POS Customers and verify outlet filter only shows customers from the active outlet."
}

main() {
    require_root
    install_possystem_binary
    restart_service_if_exists
    print_summary
}

main "$@"
