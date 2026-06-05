#!/usr/bin/env bash
set -euo pipefail

DOMAIN="${DOMAIN:-pos.zenter.co.id}"
APP_SERVICE="${APP_SERVICE:-possystem}"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd -- "${SCRIPT_DIR}/.." && pwd)"
ENV_FILE="${ROOT_DIR}/.env"
APACHE_SOURCE_CONF="${ROOT_DIR}/ops/apache/${DOMAIN}.conf"
APACHE_SITES_AVAILABLE="${APACHE_SITES_AVAILABLE:-/etc/apache2/sites-available}"
APACHE_TARGET_CONF="${APACHE_SITES_AVAILABLE}/${DOMAIN}.conf"
APACHE_SSL_SOURCE_CONF="${ROOT_DIR}/ops/apache/${DOMAIN}-le-ssl.conf"
APACHE_SSL_TARGET_CONF="${APACHE_SITES_AVAILABLE}/${DOMAIN}-le-ssl.conf"
LETSENCRYPT_DIR="${LETSENCRYPT_DIR:-/etc/letsencrypt/live/${DOMAIN}}"
APACHECTL_BIN="${APACHECTL_BIN:-}"
A2ENMOD_BIN="${A2ENMOD_BIN:-}"
A2ENSITE_BIN="${A2ENSITE_BIN:-}"
SYSTEMCTL_BIN="${SYSTEMCTL_BIN:-}"

require_root() {
    if [[ "${EUID}" -ne 0 ]]; then
        echo "Please run this installer with sudo."
        echo "Example: sudo bash ${ROOT_DIR}/ops/install-pos-zenter-production.sh"
        exit 1
    fi
}

ensure_command() {
    local command_name="$1"
    if ! command -v "${command_name}" >/dev/null 2>&1; then
        echo "Required command not found: ${command_name}"
        exit 1
    fi
}

resolve_command_path() {
    local command_name="$1"
    shift

    if command -v "${command_name}" >/dev/null 2>&1; then
        command -v "${command_name}"
        return 0
    fi

    local candidate=""
    for candidate in "$@"; do
        if [[ -x "${candidate}" ]]; then
            printf '%s\n' "${candidate}"
            return 0
        fi
    done

    return 1
}

backup_file() {
    local file_path="$1"
    if [[ -f "${file_path}" ]]; then
        cp "${file_path}" "${file_path}.bak.$(date +%Y%m%d%H%M%S)"
    fi
}

upsert_env() {
    local key="$1"
    local value="$2"

    if [[ ! -f "${ENV_FILE}" ]]; then
        touch "${ENV_FILE}"
    fi

    if grep -qE "^${key}=" "${ENV_FILE}"; then
        sed -i.bak "s|^${key}=.*|${key}=${value}|" "${ENV_FILE}"
    else
        printf '%s=%s\n' "${key}" "${value}" >> "${ENV_FILE}"
    fi
}

install_env_settings() {
    echo "Updating production env in ${ENV_FILE}"
    backup_file "${ENV_FILE}"
    upsert_env "FRONTEND_ORIGIN" "\"https://app.zenter.co.id\""
    upsert_env "POSSYSTEM_URL" "\"https://${DOMAIN}\""
    upsert_env "APIFITNESS_API_URL" "\"https://v2.zenter.co.id\""
}

enable_apache_modules() {
    echo "Enabling Apache proxy modules"
    "${A2ENMOD_BIN}" proxy proxy_http headers rewrite ssl >/dev/null
}

install_apache_site() {
    if [[ ! -f "${APACHE_SOURCE_CONF}" ]]; then
        echo "Apache source config not found: ${APACHE_SOURCE_CONF}"
        exit 1
    fi

    mkdir -p "${APACHE_SITES_AVAILABLE}"
    backup_file "${APACHE_TARGET_CONF}"
    cp "${APACHE_SOURCE_CONF}" "${APACHE_TARGET_CONF}"

    echo "Enabling Apache site ${DOMAIN}"
    "${A2ENSITE_BIN}" "${DOMAIN}.conf" >/dev/null

    if [[ -f "${APACHE_SSL_SOURCE_CONF}" ]]; then
        backup_file "${APACHE_SSL_TARGET_CONF}"
        cp "${APACHE_SSL_SOURCE_CONF}" "${APACHE_SSL_TARGET_CONF}"

        if [[ -s "${LETSENCRYPT_DIR}/fullchain.pem" && -s "${LETSENCRYPT_DIR}/privkey.pem" ]]; then
            echo "Enabling Apache SSL site ${DOMAIN}-le-ssl"
            "${A2ENSITE_BIN}" "${DOMAIN}-le-ssl.conf" >/dev/null
        else
            echo "SSL certificate for ${DOMAIN} not found yet."
            echo "Run: sudo certbot certonly --apache -d ${DOMAIN}"
            echo "Then rerun: sudo bash ${ROOT_DIR}/ops/install-pos-zenter-production.sh"
        fi
    fi
}

reload_apache() {
    echo "Testing Apache configuration"
    "${APACHECTL_BIN}" configtest

    echo "Reloading apache2"
    "${SYSTEMCTL_BIN}" reload apache2
}

restart_app_service_if_available() {
    if "${SYSTEMCTL_BIN}" list-unit-files | grep -q "^${APP_SERVICE}\\.service"; then
        echo "Restarting ${APP_SERVICE}.service"
        "${SYSTEMCTL_BIN}" restart "${APP_SERVICE}"
    else
        echo "Service ${APP_SERVICE}.service not found. Restart it manually if needed."
    fi
}

print_summary() {
    echo
    echo "Installation complete for ${DOMAIN}"
    echo "- Env updated: ${ENV_FILE}"
    echo "- Apache config: ${APACHE_TARGET_CONF}"
    if [[ -f "${APACHE_SSL_TARGET_CONF}" ]]; then
        echo "- Apache SSL config: ${APACHE_SSL_TARGET_CONF}"
    fi
    echo
    echo "Suggested checks:"
    echo "  systemctl status apache2 --no-pager -l"
    echo "  systemctl status ${APP_SERVICE} --no-pager -l"
}

main() {
    require_root
    ensure_command grep
    ensure_command sed

    APACHECTL_BIN="$(resolve_command_path apache2ctl /usr/sbin/apache2ctl /usr/local/apache2/bin/apachectl)" || {
        echo "Required command not found: apache2ctl"
        echo "Install apache2 first, or run with APACHECTL_BIN=/full/path/to/apachectl"
        exit 1
    }

    A2ENMOD_BIN="$(resolve_command_path a2enmod /usr/sbin/a2enmod)" || {
        echo "Required command not found: a2enmod"
        exit 1
    }

    A2ENSITE_BIN="$(resolve_command_path a2ensite /usr/sbin/a2ensite)" || {
        echo "Required command not found: a2ensite"
        exit 1
    }

    SYSTEMCTL_BIN="$(resolve_command_path systemctl /usr/bin/systemctl /bin/systemctl)" || {
        echo "Required command not found: systemctl"
        exit 1
    }

    install_env_settings
    enable_apache_modules
    install_apache_site
    reload_apache
    restart_app_service_if_available
    print_summary
}

main "$@"
