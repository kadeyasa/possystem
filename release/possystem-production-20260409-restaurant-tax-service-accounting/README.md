possystem production package
Release: 2026-04-09
Slug: restaurant-tax-service-accounting

Contents:
- possystem-linux-amd64
- possystem-linux-arm64
- ops/install-pos-zenter-production.sh
- ops/apache/pos.zenter.co.id.conf
- ops/apache/pos.zenter.co.id-le-ssl.conf

Included changes:
- Restaurant tax and service accounting support
- POS draft and transaction totals with service charge
- Journal split for sales, service, and tax
- Shift closing service totals

Suggested production target:
- App directory: /var/www/applications/v2/possystem
- Binary path: /var/www/applications/v2/possystem/possystem
