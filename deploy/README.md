# Deploying btcdwatch.com

Runbook for the public single-box deployment: **btcd (mainnet) + btcdwatchd +
Caddy** on one server, Cloudflare in front. Everything here is manual and
tag-based by design — no CI/CD, no deploy credentials anywhere.

```
 browser ──▶ Cloudflare (DNS, DDoS, edge cache, WAF)
                 │  HTTPS (Full strict)
                 ▼
             Caddy :443 (auto-TLS, HSTS)
                 │  127.0.0.1:8480
                 ▼
             btcdwatchd (rate limits, scan caps — see config)
                 │  127.0.0.1:8334 (websocket RPC, TLS)
                 ▼
             btcd mainnet (txindex + addrindex)
```

## 1. Server preparation

Two system users so the explorer can't touch the chain data:

```sh
sudo useradd -r -m -d /var/lib/btcd -s /usr/sbin/nologin btcd
sudo useradd -m -s /bin/bash btcdwatch
sudo mkdir -p /var/lib/btcd /etc/btcd /etc/btcdwatch
sudo chown btcd:btcd /var/lib/btcd
```

Install: Go 1.25+, Node 22+, git, Caddy (distro package), and btcd 0.26+
(build from source: `go install github.com/btcsuite/btcd@v0.26.0`, then copy
to `/usr/local/bin`).

## 2. btcd

```sh
sudo cp deploy/btcd.conf /etc/btcd/btcd.conf
# append generated credentials (server-only, never committed):
echo "rpcuser=btcdwatch"                     | sudo tee -a /etc/btcd/btcd.conf
echo "rpcpass=$(openssl rand -hex 32)"       | sudo tee -a /etc/btcd/btcd.conf
sudo chown root:btcd /etc/btcd/btcd.conf && sudo chmod 640 /etc/btcd/btcd.conf

sudo cp deploy/btcd.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now btcd
```

Initial mainnet sync with both indexes takes days; watch it with
`journalctl -fu btcd`. The explorer can be installed meanwhile — it serves
`503 node_unavailable` until the node is ready.

Make the RPC cert readable by the explorer once it exists:

```sh
sudo chgrp btcdwatch /var/lib/btcd/rpc.cert && sudo chmod 640 /var/lib/btcd/rpc.cert
```

(The cert is regenerated only if deleted; if that happens, redo this.)

## 3. btcdwatchd

```sh
sudo -iu btcdwatch git clone https://github.com/saubyk/btcdwatch.com.git
sudo cp deploy/config.yaml.example /etc/btcdwatch/config.yaml
sudo vi /etc/btcdwatch/config.yaml   # paste the rpcuser/rpcpass from step 2
sudo chown root:btcdwatch /etc/btcdwatch/config.yaml && sudo chmod 640 /etc/btcdwatch/config.yaml

sudo cp deploy/btcdwatchd.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable btcdwatchd
```

First deploy (and every upgrade) is tags-only:

```sh
sudo -iu btcdwatch bash -c 'cd btcdwatch.com && ./deploy/upgrade.sh v1.0.0'
```

`upgrade.sh` refuses dirty trees and unknown tags, keeps the previous binary,
health-checks after restart, and auto-rolls-back if the new binary is
unhealthy. Manual rollback: `./deploy/upgrade.sh --rollback`.

## 4. Caddy

```sh
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

## 5. Cloudflare + firewall

1. Point the domain's nameservers at Cloudflare (registration stays put);
   create an A record for `btcdwatch.com` (+ `www`) to the server IP,
   **proxied** (orange cloud).
2. SSL/TLS mode: **Full (strict)**. Enable WebSockets (Network tab).
3. Firewall — the whole point of the proxy is that nobody talks to the
   origin directly:

```sh
sudo ufw default deny incoming
sudo ufw allow ssh          # tighten to your own IP if it's static
sudo ufw allow 8333/tcp     # btcd p2p (optional but good citizenship)
# 443/80 from Cloudflare only (https://www.cloudflare.com/ips/):
for ip in $(curl -s https://www.cloudflare.com/ips-v4) \
          $(curl -s https://www.cloudflare.com/ips-v6); do
    sudo ufw allow from "$ip" to any port 443 proto tcp
    sudo ufw allow from "$ip" to any port 80  proto tcp
done
sudo ufw enable
```

The app reads the client IP from `CF-Connecting-IP`
(`trusted_proxy_header` in the config). **These two settings travel
together**: if Cloudflare is ever removed, open 443 to the world *and*
empty `trusted_proxy_header`, or rate limiting can be spoofed.

## 6. Verify

```sh
curl -s https://btcdwatch.com/api/healthz
# {"status":"ok","network":"mainnet","nodeConnected":true,"blockHeight":...}
```

Then walk the E2E recipe in the top-level README against mainnet: search a
recent txid, an address, a block; open the site and confirm the live
mempool queue moves and Watch mode connects (WS through Cloudflare).

## 7. Day-2 operations

- **Upgrades**: tag a release on GitHub → `./deploy/upgrade.sh <tag>`.
- **Monitoring**: point an uptime monitor (e.g. UptimeRobot) at
  `https://btcdwatch.com/api/healthz` — it returns 503 whenever the node
  connection is down, so one check covers both services. Watch disk on
  `/var/lib/btcd` (chain + indexes grow steadily) and set an alert at 85%.
- **Logs**: `journalctl -fu btcdwatchd`, `journalctl -fu btcd`.
- **btcd upgrades**: stop `btcdwatchd` first, upgrade/restart `btcd`, wait
  for `healthz` to report reconnect. Never `kill -9` btcd — index recovery
  is expensive (the unit's `TimeoutStopSec=600` exists for this).
- **Price API**: CoinGecko's free tier may throttle a busy site; the app
  falls back to the labeled static price automatically. If it happens
  often, get a CoinGecko API key (future work: config support for it).
