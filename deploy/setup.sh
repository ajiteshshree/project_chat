#!/usr/bin/env bash
set -euo pipefail

# One-shot bootstrap for an Ubuntu 22.04+ / Debian VM.
# Steps done beforehand (see DEPLOY.md):
#   1. Oracle Cloud console: open TCP 80 and 443 in the VCN security list.
#   2. DuckDNS: create a subdomain and point it at this VM's public IP.
#   3. SSH in and run:  bash deploy/setup.sh

cd "$(dirname "$0")"

echo "== Project Chat setup =="

read -rp "DuckDNS subdomain (e.g. mychat.duckdns.org): " APP_DOMAIN
read -rp "ACME email for HTTPS certificates (you@example.com): " ACME_EMAIL

if ! command -v docker >/dev/null 2>&1; then
  echo "Installing Docker…"
  sudo apt-get update
  sudo apt-get install -y docker.io docker-compose-v2
  sudo systemctl enable --now docker
fi

# Allow HTTP/S through the host firewall (ignores Oracle's *network* firewall,
# which must be opened in the VCN security list separately).
if command -v ufw >/dev/null 2>&1; then
  sudo ufw allow 80/tcp >/dev/null 2>&1 || true
  sudo ufw allow 443/tcp >/dev/null 2>&1 || true
fi

PW_POSTGRES="$(openssl rand -hex 16)"
PW_MINIO="$(openssl rand -hex 16)"
JWT="$(openssl rand -hex 48)"
MINIO_USER="minio-$(echo "$APP_DOMAIN" | cut -d. -f1)"

cp .env.example .env
sed -i "s|^APP_DOMAIN=.*|APP_DOMAIN=${APP_DOMAIN}|" .env
sed -i "s|^ACME_EMAIL=.*|ACME_EMAIL=${ACME_EMAIL}|" .env
sed -i "s|^JWT_SECRET=.*|JWT_SECRET=${JWT}|" .env
sed -i "s|^DATABASE_URL=.*|DATABASE_URL=postgres://chat:${PW_POSTGRES}@postgres:5432/chatdb|" .env
sed -i "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=${PW_POSTGRES}|" .env
sed -i "s|^MINIO_ROOT_USER=.*|MINIO_ROOT_USER=${MINIO_USER}|" .env
sed -i "s|^MINIO_ROOT_SECRET=.*|MINIO_ROOT_SECRET=${PW_MINIO}|" .env

echo "Starting services (first build downloads images & compiles; takes a few minutes)…"
docker compose up -d --build

echo
echo "Done. Your chat should be live at https://${APP_DOMAIN} shortly (up to a minute for the TLS cert)."
echo "Check status:  docker compose ps"
echo "Watch logs:    docker compose logs -f app"