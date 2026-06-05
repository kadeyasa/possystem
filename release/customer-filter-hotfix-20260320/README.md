# POS Customer Filter Hotfix

Bundle ini berisi update backend `possystem` untuk customer list agar support filter `outlet_id`, search, pagination, dan soft delete handling yang dipakai `adminfitness`.

Isi bundle:

- `possystem-linux-amd64`
- `deploy-production.sh`

## Upload

```bash
scp customer-filter-hotfix-20260320.tgz admin@SERVER_IP:/var/www/applications/v2/possystem/
```

## Deploy

Asumsi path production:

- `possystem`: `/var/www/applications/v2/possystem`

Perintah default:

```bash
cd /var/www/applications/v2/possystem
tar -xzf customer-filter-hotfix-20260320.tgz
cd customer-filter-hotfix-20260320
sudo bash deploy-production.sh
```

## Verifikasi

```bash
systemctl status possystem --no-pager -l
curl -k -I https://pos.zenter.co.id
```

Functional check:

- buka `adminfitness > POS Customers`
- pindah outlet aktif
- pastikan request customer mengirim `outlet_id`
- pastikan list customer berubah mengikuti outlet
