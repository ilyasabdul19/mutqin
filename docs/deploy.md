# Deployment Notes

This document covers the manual steps to deploy Mutqin to a production VPS using Docker Compose and Traefik with Let's Encrypt wildcard TLS via Cloudflare DNS-01.

## Prerequisites

- A VPS (e.g., Hetzner CX22) running Ubuntu 24.04 with Docker + Docker Compose v2 installed.
- A registered domain (`mutqin.app`) with DNS managed in Cloudflare.
- A Cloudflare API token with `Zone:DNS:Edit` permission on the zone.
- The wildcard A record `*.mutqin.app` set up in Cloudflare (gray-cloud / DNS-only is fine on the Free plan, or per-tenant proxied records auto-created later by Plan G's onboarding utility).

## One-time host setup

```sh
# On the VPS:
mkdir -p /opt/mutqin/letsencrypt
cd /opt/mutqin
git clone https://github.com/<owner>/mutqin.git src
cd src
```

## Required env vars

Create `/opt/mutqin/.env`:

```
DATABASE_URL=postgres://mutqin:<strongpw>@db:5432/mutqin?sslmode=disable
APP_DATABASE_URL=postgres://mutqin_app:<strongpw_app>@db:5432/mutqin?sslmode=disable
JWT_SECRET=<long-random>
BASE_HOST=mutqin.app
CF_DNS_API_TOKEN=<cloudflare token>
ACME_EMAIL=ops@mutqin.app
```

(`BASE_HOST` differs from local: tenant subdomains resolve via `*.mutqin.app` instead of `*.localhost`.)

## Production compose overrides

Use `docker-compose.prod.yml` (TODO — Plan F adds this) to override:
- The traefik volume mount → `infra/traefik/traefik.prod.yml`
- An additional volume `letsencrypt:/letsencrypt` for ACME persistence
- Open port `443` in addition to `80`

Until Plan F lands, you can manually adjust `docker-compose.yml` on the host or run a one-line override:

```sh
docker run -d --name mutqin-traefik \
  --network mutqin \
  -p 80:80 -p 443:443 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v $(pwd)/infra/traefik/traefik.prod.yml:/etc/traefik/traefik.yml:ro \
  -v /opt/mutqin/letsencrypt:/letsencrypt \
  -e CF_DNS_API_TOKEN=$CF_DNS_API_TOKEN \
  -e ACME_EMAIL=$ACME_EMAIL \
  traefik:v3.1
```

## First TLS issuance

The first request to any host after starting Traefik triggers ACME DNS-01:

1. Traefik writes a `_acme-challenge.<domain>` TXT record via Cloudflare.
2. Let's Encrypt validates the record and issues a wildcard cert covering `mutqin.app` + `*.mutqin.app`.
3. The cert is stored in `/letsencrypt/acme.json` (chmod 600 on first write).

Watch the logs:

```sh
docker logs -f mutqin-traefik | grep -i acme
```

A successful run logs `[INFO] ... Server responded with a certificate.`

## Renewal

Let's Encrypt issues 90-day certs. Traefik auto-renews ~30 days before expiry. No cron job needed.

## Backups

`pgdata` and `letsencrypt` volumes are the persistent state. Schedule:

```sh
0 3 * * * docker run --rm -v mutqin_pgdata:/data postgres:16-alpine pg_dump -U mutqin mutqin | gzip > /opt/backups/mutqin-$(date +\%F).sql.gz
```

(Adjust path for your VPS. Cloudflare R2 sync is in Plan G alongside onboarding automation.)
