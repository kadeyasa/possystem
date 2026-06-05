# POS Variant Durasi Hotfix

Bundle ini berisi hotfix backend `possystem` untuk kasus:

`json: cannot unmarshal number into Go struct field Variant.durasi of type string`

Perbaikan membuat endpoint item/variant menerima `durasi` baik sebagai angka maupun string, sehingga form `adminfitness > POS Service Variants` bisa menyimpan data normal lagi.

Isi bundle:

- `possystem-linux-amd64`
- `possystem-linux-arm64`
- `deploy-production.sh`

## Upload

```bash
scp possystem-variant-durasi-hotfix-20260321.tgz admin@SERVER_IP:/var/www/applications/v2/possystem/
```

## Deploy

Asumsi path production:

- `possystem`: `/var/www/applications/v2/possystem`

Perintah default:

```bash
cd /var/www/applications/v2/possystem
tar -xzf possystem-variant-durasi-hotfix-20260321.tgz
cd possystem-variant-durasi-hotfix-20260321
sudo bash deploy-production.sh
```

Script deploy akan:

- mendeteksi arsitektur server (`amd64` atau `arm64`)
- backup binary lama `possystem`
- install binary baru ke `/var/www/applications/v2/possystem/possystem`
- restart service `possystem` jika ada

## Verifikasi

```bash
systemctl status possystem --no-pager -l
ls -lh /var/www/applications/v2/possystem/possystem
curl -k -I https://pos.zenter.co.id
```

Functional check:

- buka `adminfitness > POS Service Variants`
- tambah atau edit variant dengan `durasi` angka, misalnya `3`
- pastikan simpan berhasil dan error unmarshal tidak muncul lagi
