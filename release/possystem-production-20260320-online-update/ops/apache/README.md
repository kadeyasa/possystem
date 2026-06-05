Use these Apache vhost configs for the production POS API domain `pos.zenter.co.id`.

Fastest install on the server:

```bash
cd /var/www/applications/v2/possystem
sudo bash ops/install-pos-zenter-production.sh
```

HTTP traffic is redirected to HTTPS, except `/.well-known/acme-challenge/` so Certbot can issue the certificate.

The installer will automatically enable the SSL vhost when the Let's Encrypt certificate already exists.

If the certificate does not exist yet, run:

```bash
sudo certbot certonly --apache -d pos.zenter.co.id
sudo bash ops/install-pos-zenter-production.sh
```
