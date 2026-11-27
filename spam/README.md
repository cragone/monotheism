# spam — spam.charlesragone.com

A Threads-style social app: top-level threads, nested replies, likes, follows,
profiles. Go (stdlib `net/http`) + SQLite (`modernc.org/sqlite`, pure Go) on the
back, React + Vite + Tailwind + daisyUI on the front.

## Accounts (copied from super-chat)

There is no signup, login or password.

- An account is 32 random bytes generated on the device, shown as a 24-word
  BIP39 recovery phrase (Account page). Import the phrase on another device to
  get the same account back.
- An Ed25519 key is derived from the seed. Every request is signed
  (`X-Spam-Key`, `X-Spam-Ts`, `X-Spam-Nonce`, `X-Spam-Sig` over
  `METHOD\nPATH?QUERY\nTS\nNONCE\nsha256(body)`); the server verifies the
  signature, a ±5 min clock window, and rejects replays.
- The seed never leaves the browser. The server only ever sees the public key;
  claiming a `@username` (first signed `PUT /api/me`) creates the profile.
- Losing the phrase = losing the account; there is no reset.

## Layout

- `server/` — Go API + static file server (`cmd/server`). SQLite file at
  `$DATA_DIR/spam.sqlite`. Env: `PORT` (8080), `DATA_DIR` (./data),
  `STATIC_DIR` (./web/dist).
- `src/` — React client.
- `scripts/` — deploy scripts for the bw-server k3s box.

## Local dev

```bash
cd spam/server && go run ./cmd/server        # :8080
cd spam && npm install && npm run dev        # Vite, proxies /api to :8080
```

## Deploy (local bw server, like vecna)

Runs on the home bw-server's k3s, reached through the Cloudflare Tunnel.
`spam/scripts/deploy-local.sh` builds the image on the host, imports it into
k3s, applies `kubernetes/spam-production.yaml` + `kubernetes/spam-ingress.yaml`,
and rolls out. CI (`production_spam.yml`) triggers it via the deploy webhook.

One-time host setup (outside this repo):

1. Cloudflare Tunnel `~/.cloudflared/config.yml`: add an ingress rule
   `hostname: spam.charlesragone.com` → `service: http://localhost:80`, plus a
   proxied DNS CNAME for `spam` to the tunnel
   (`cloudflared tunnel route dns <tunnel> spam.charlesragone.com`), then
   restart the `cloudflared-tunnel` unit.
2. Webhook `vecna/scripts/hooks.json` on the host: add a second hook
   `deploy-spam` (copy the `deploy-vecna` entry, same token) with
   `execute-command` = `/home/charlie/monotheism/spam/scripts/deploy-local.sh`;
   restart the `vecna-deploy-webhook` unit.
3. First deploy by hand: `spam/scripts/deploy-bw-server.sh`.

The SQLite PVC is `ReadWriteOnce`, so keep `prod-spam` at one replica.
