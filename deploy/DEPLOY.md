# DEPLOY.md — production deployment runbook

Target: Hostinger KVM 2 · Debian 12 · Docker Compose · Caddy (automatic HTTPS)
App hostname: `delta-attendance.cloud`
Deploy branch: `main`

Run every command as **root** on the server unless noted. Everything on your laptop is marked **LOCAL**.

---

## 0. Prerequisites (have these ready)

- [ ] A **Firebase project** and its service account JSON (`firebase/service-account.json`) for the SAME project your mobile app uses for Firebase Auth. **Without this, login returns 401 in production.**
- [ ] The repo pushed to GitHub (branch `main`) so the server can clone it.
- [ ] Your mobile app's API base URL setting, so we can point it at `https://delta-attendance.cloud`.

---

## 1. Server basics (verify these exist; fix if missing)

```bash
# Security updates
apt update && apt upgrade -y
apt install -y unattended-upgrades && dpkg-reconfigure -plow unattended-upgrades

# Swap file (2 GB) on an 8 GB box as OOM insurance
fallocate -l 2G /swapfile && chmod 600 /swapfile
mkswap /swapfile && swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab

# Timezone
timedatectl set-timezone UTC

# Firewall: only 22, 80, 443 (Caddy). 8080 must NOT be public.
apt install -y ufw
ufw allow 22/tcp && ufw allow 80/tcp && ufw allow 443/tcp
ufw --force enable
```

```bash
# Docker + compose plugin (official Docker repo for Debian)
apt install -y ca-certificates curl
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian bookworm stable" > /etc/apt/sources.list.d/docker.list
apt update && apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
systemctl enable --now docker
```

---

## 2. Get the code on the server

```bash
cd /root
git clone https://github.com/coderGtm/delta.git && cd delta
git checkout main
git pull --ff-only origin main
```

---

## 3. Create secrets (`.env`)

```bash
cd /root/delta
cp .env.production.example .env
chmod 600 .env
nano .env
```

Fill in (generate values with `openssl rand -hex 32`):
- `POSTGRES_PASSWORD` — strong DB password
- `JWT_SECRET` — `openssl rand -hex 32`
- `PROMETHEUS_BEARER_TOKEN` — `openssl rand -hex 32`
- `GRAFANA_ADMIN_PASSWORD` (only if running Grafana from compose)
- Leave `TRUST_PROXY_HEADERS=true`, `LOG_FORMAT=json`, `AUTO_MIGRATE=true`

---

## 4. Firebase service account

```bash
mkdir -p /root/delta/firebase
```

**LOCAL** — upload your service account (one of):
```bash
scp firebase/service-account.json root@<SERVER_IP>:/root/delta/firebase/service-account.json
# or
rsync -av firebase/ root@<SERVER_IP>:/root/delta/firebase/
```

Verify on the server:
```bash
test -s /root/delta/firebase/service-account.json && echo "firebase sa present"
```

---

## 5. Prometheus scrape token (must match `.env` PROMETHEUS_BEARER_TOKEN)

```bash
cd /root/delta
# Use the SAME value you put in .env:
echo "PASTE_THE_PROMETHEUS_TOKEN_HERE" > monitoring/prometheus/prometheus-token.txt
chmod 600 monitoring/prometheus/prometheus-token.txt
```

---

## 6. Start the stack

```bash
cd /root/delta
docker compose up --build -d postgres app prometheus
docker compose ps
```

First build takes a few minutes. Migrations run automatically (`AUTO_MIGRATE=true`).

---

## 7. Verify

```bash
# Health/readiness through Caddy (public)
curl -fsS https://delta-attendance.cloud/healthz      # {"status":"UP"}
curl -fsS https://delta-attendance.cloud/readyz       # {"status":"UP"}

# OpenAPI docs
curl -fsS https://delta-attendance.cloud/docs/ | head

# Metrics (token-gated)
TOKEN=$(cat /root/delta/monitoring/prometheus/prometheus-token.txt)
curl -fsS -H "Authorization: Bearer $TOKEN" https://delta-attendance.cloud/metrics | grep -E 'delta_app|go_goroutines' | head

# Containers healthy
docker compose ps
```

**Crucial real-auth check:** sign in on your mobile app (or via the login endpoint) with a real Firebase user. If login returns `INVALID_TOKEN`, the Firebase service account is wrong/missing.

---

## 8. Backups (daily, automated)

```bash
cd /root/delta
chmod +x deploy/backup.sh deploy/restore.sh
mkdir -p /var/backups/delta

# Test a manual backup
./deploy/backup.sh

# Schedule daily at 02:00
(crontab -l 2>/dev/null; echo "0 2 * * * /root/delta/deploy/backup.sh >> /var/log/delta-backup.log 2>&1") | crontab -
```

**Off-site copy (recommended, ₹0):** Backblaze B2 free tier (10 GB) + rclone:
```bash
apt install -y rclone
rclone config   # choose Backblaze B2, add your keys
# then set the remote in the script:
RCLONE_REMOTE=b2:delta-backups ./deploy/backup.sh
```

**Restore drill (documented):**
```bash
# Stop writes, then:
./deploy/restore.sh /var/backups/delta/delta-<TIMESTAMP>.sql.gz
```

---

## 9. Monitoring (Prometheus on the box, Grafana on your laptop)

Prometheus is already running from compose and scrapes `app:8080/metrics` continuously — full history stays on the box.

**LOCAL** — run Grafana on your laptop and tunnel to the box's Prometheus:
```bash
# On the server, prometheus listens on 9090. Tunnel it:
ssh -N -L 9090:127.0.0.1:9090 root@<SERVER_IP>
```
Then add a Prometheus datasource in local Grafana → `http://localhost:9090`, and import the dashboard from this repo: `monitoring/grafana/dashboards/delta-overview.json`.

**Uptime alerts (₹0):** create an UptimeRobot free monitor → `https://delta-attendance.cloud/healthz` (alert if down >2 min).

---

## 10. Go-live: point the mobile app at production

1. In the mobile app, set the API base URL to `https://delta-attendance.cloud`.
2. Confirm the full flow over the live endpoint: Firebase login → create outlet → clock in/out → salary report.
3. Submit to the Play Store.

---

## Troubleshooting

| Symptom | Likely cause / fix |
|---|---|
| login → `INVALID_TOKEN` | Firebase service account missing/wrong at `firebase/service-account.json` |
| `/readyz` DOWN | Postgres not healthy; check `docker compose logs postgres` |
| metrics 403 | `monitoring/prometheus/prometheus-token.txt` doesn't match `.env` |
| app container crash-loops | `docker compose logs app`; JWT_SECRET unset? DB password mismatch in `.env`? |
| client IPs wrong in logs/rate limits | `TRUST_PROXY_HEADERS` must be `true` behind Caddy |
| cert issues | DNS A record must point at the VPS; ports 80/443 open; `journalctl -u caddy -f` |

## Updates / redeploys

```bash
cd /root/delta
git pull --ff-only origin main
docker compose up --build -d postgres app prometheus
```