# Deployment guide — Oracle Always Free VM (or any Ubuntu 22.04+ VPS)

Goal: a 24/7 chat website at `https://yourname.duckdns.org` for $0/month.

## Things you need (all free)

1. **Oracle Cloud Always Free account** — https://www.oracle.com/cloud/free/
   (a credit card is asked for identity verification but nothing is charged).
2. **DuckDNS account + one free subdomain** — https://www.duckdns.org
3. A GitHub account to hold the repo (free).

## 1. Create the VM

- Compute → Instances → Create instance.
- Image: **Ubuntu 22.04+** (Oracle Linux 8 also works with `firewalld`).
- Shape: use an **Always Free eligible** shape (Ampere A1 or E2 micro).
- Boot volume: ~100–200 GB (Always Free allocates up to 200 GB total).
- Generate/upload an SSH key pair and save the private key.

## 2. Open ports in the Oracle firewall

AWS-style network firewalls block ports even if the OS allows them:

- VCN → your subnet → **Security List** → add Ingress rules:
  - TCP **80** (HTTP) — needed for the HTTPS certificate challenge
  - TCP **443** (HTTPS)
  - TCP **22** (SSH, usually already open)
- Egress "allow all" is the default — leave it.

## 3. Point DuckDNS at the VM

- In DuckDNS, create a subdomain (e.g. `mychat` → `mychat.duckdns.org`).
- Set the **A record** to the VM's public IPv4 address.
- (Optional) cron on some machine to refresh the IP; for a static Oracle
  instance the public IP doesn't change unless you stop/start it.

## 4. Install and run

```bash
# SSH in:  ssh -i ~/.ssh/oracle_key ubuntu@<public-ip>

sudo apt-get update && sudo apt-get install -y git
git clone https://github.com/<you>/project-chat && cd project-chat
bash deploy/setup.sh
```

`setup.sh` asks for your DuckDNS subdomain + email, then:

- installs Docker,
- opens ports 80/443 in ufw (the OS firewall),
- generates a random `JWT_SECRET`, Postgres and MinIO passwords straight into `deploy/.env`,
- builds and starts everything: `app` (Go), `postgres`, `minio`, `caddy`.

Wait ~1 minute for the TLS certificate. Visit **https://mychat.duckdns.org**.

## Day-to-day

```bash
docker compose -f deploy/docker-compose.yml ps        # status
docker compose -f deploy/docker-compose.yml logs -f app   # backend logs
docker compose -f deploy/docker-compose.yml down      # stop

# update the app after a `git pull`:
git pull
docker compose -f deploy/docker-compose.yml up -d --build
```

## Admin: MinIO console

The console (port 9001) is intentionally not exposed. Tunnel it over SSH:

```bash
ssh -L 9001:localhost:9001 -i ~/.ssh/oracle_key ubuntu@<public-ip>
# then open http://localhost:9001 in your browser
# user/pass are MINIO_ROOT_USER / MINIO_ROOT_SECRET from deploy/.env
```

## Backups (important — this is a single VM)

```bash
# Postgres → SQL dump
docker compose -f deploy/docker-compose.yml exec postgres \
  pg_dump -U chat chatdb | gzip > chat-backup-$(date +%F).sql.gz

# MinIO objects → mirror to anywhere (another machine, R2, …)
docker compose -f deploy/docker-compose.yml exec minio \
  mc mirror --overwrite local/chat-files /tmp/chat-files
```

Restore: `pg_dump` → `psql`, and `mc mirror` back into the bucket.

## Troubleshooting

| Symptom | Fix |
|---|---|
| Certificate never issues | Port 80 must be open in BOTH the Oracle Security List and the OS firewall; DuckDNS A record must point to this VM's public IP. |
| `app` container restart-looping | Check `docker compose logs app`. Usually Postgres/MinIO not healthy yet — they have health checks; the app waits for them. |
| Images/videos don't load | Check `docker compose logs -f app` for MinIO errors; confirm `MINIO_ENDPOINT=minio:9000` (internal Docker name) in `.env`. |
| Site up but no live messages | WebSocket blocked — ensure nothing proxies `/ws` weirdly; Caddy reverse-proxies it fine. |
| Can't SSH | In the Oracle console, confirm the SSH key matches the instance. |

## Reset everything

```bash
docker compose -f deploy/docker-compose.yml down -v   # deletes DB + MinIO volumes + certs
bash deploy/setup.sh                                  # start over fresh
```