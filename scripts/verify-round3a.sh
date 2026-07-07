#!/usr/bin/env bash
# Live verification of round 3 PR A (live mempool layer) against the
# regtest harness. Assumes the server on :8480 with miner/txgen churning.
set -u

API="${BTCDWATCH_URL:-http://127.0.0.1:8480}"
pass=0 fail=0

check() { # check <exit-code> <label> <detail>
  if [ "$1" = 0 ]; then pass=$((pass + 1)); echo "  ✓ $2  $3"
  else fail=$((fail + 1)); echo "  ✗ $2  $3"; fi
}

get() { # get <url> <dot.path>
  curl -sf "$1" | python3 -c "
import sys, json, functools
try:
    d = json.load(sys.stdin)
    v = functools.reduce(
        lambda o, k: o[int(k)] if isinstance(o, list) else o[k],
        '$2'.split('.'), d)
    print(json.dumps(v) if isinstance(v, (dict, list, bool)) else v)
except Exception:
    pass" 2>/dev/null
}

echo "— queue gains the capacity-track peak —"
# Right after a server restart /api/stats can 503 while the btcd
# websocket session is still coming up — retry for up to ~15s.
total="" peak=""
for _ in $(seq 1 15); do
  total=$(get "$API/api/stats" queue.totalVbytes)
  peak=$(get "$API/api/stats" queue.peakVbytes)
  [ -n "$peak" ] && break
  sleep 1
done
[ -n "$peak" ] && [ "$peak" -ge "${total:-0}" ] 2>/dev/null
check $? "queue.peakVbytes present, ≥ total" "total=$total peak=$peak"

echo "— WS live layer (mempool + block events) —"
python3 - "$API" <<'PYEOF'
import base64, json, os, socket, sys, time, urllib.parse

url = urllib.parse.urlparse(sys.argv[1])
host, port = url.hostname, url.port or 80
key = base64.b64encode(os.urandom(16)).decode()

s = socket.create_connection((host, port), timeout=5)
s.sendall((
    f"GET /api/ws HTTP/1.1\r\nHost: {host}:{port}\r\n"
    "Upgrade: websocket\r\nConnection: Upgrade\r\n"
    f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n\r\n"
).encode())

buf = b""
while b"\r\n\r\n" not in buf:
    buf += s.recv(4096)
_, buf = buf.split(b"\r\n\r\n", 1)

def frames(deadline):
    global buf
    while time.time() < deadline:
        while len(buf) >= 2:
            b1, b2 = buf[0], buf[1]
            ln, off = b2 & 0x7F, 2
            if ln == 126:
                if len(buf) < 4: break
                ln, off = int.from_bytes(buf[2:4], "big"), 4
            elif ln == 127:
                if len(buf) < 10: break
                ln, off = int.from_bytes(buf[2:10], "big"), 10
            if len(buf) < off + ln: break
            payload, buf = buf[off:off + ln], buf[off + ln:]
            if b1 & 0x0F == 1:
                yield payload
        s.settimeout(max(0.2, deadline - time.time()))
        try:
            chunk = s.recv(65536)
        except socket.timeout:
            return
        if not chunk:
            return
        buf += chunk

seen = {}
# 25s window: enough for connect pushes, a 2s-throttled mempool push from
# txgen churn, and usually a block event (~60s interval — may miss).
for payload in frames(time.time() + 25):
    try:
        msg = json.loads(payload)
    except Exception:
        continue
    t = msg.get("type")
    seen.setdefault(t, msg)
    if {"stats", "mempool", "block"} <= seen.keys():
        break

ok = lambda cond, label, detail: print(f"  {'✓' if cond else '✗'} {label}  {detail}")

mp = seen.get("mempool", {}).get("data", {})
q = mp.get("queue") or {}
arr = mp.get("arrivals")
ok("mempool" in seen, "WS mempool push", f"txCount={q.get('txCount')} peak={q.get('peakVbytes')}")
ok(isinstance(arr, list), "arrivals list present", f"n={len(arr) if isinstance(arr, list) else '-'}")
if isinstance(arr, list) and arr:
    a = arr[0]
    ok(all(k in a for k in ("txid", "amountSats", "feeRateSatPerVb", "vsize", "time")),
       "arrival fields", f"rate={a.get('feeRateSatPerVb')} vsize={a.get('vsize')}")
if "block" in seen:
    b = seen["block"]["data"]
    ok(True, "WS block flash", f"height={b.get('height')} txCount={b.get('txCount')}")
else:
    print("  (no block mined inside the 25s window — flash is a UI check)")
sys.exit(0 if "mempool" in seen else 1)
PYEOF
check $? "live WS layer" ""

echo
echo "RESULT: $pass passed, $fail failed  (plus the WS lines above)"
echo
echo "Manual UI spot-checks at $API:"
echo "  1. 'The line right now': bar sits on a dashed track and visibly grows as txgen churns;"
echo "     count next to 'Live ·' ticks up in mono."
echo "  2. On a mined block: green '⛏️ Block N just mined — M transactions left…' banner ~6s,"
echo "     bar contracts with a smooth 0.9s transition, cutoff marker shifts."
echo "  3. 'Just joined the line': rows appear (newest on top, capped 6) with band dot, txid,"
echo "     age, rate, ETA chip, BTC amount; clicking a row opens its Pending view."
echo "  4. Caption reads 'Whole line clears in ~N blocks (≈M min) · dashed track = recent peak'"
echo "     with regtest-realistic minutes (~1 min/block), not 10-min-block math."
echo "  5. Dark stats bar 'Mempool size' ticks with the same feed."
