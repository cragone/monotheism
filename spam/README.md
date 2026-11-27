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

## Deploy (Azure VM k8s cluster)

Same pipeline as charlesragone.com: `.github/workflows/production_spam.yml`
SSHes to the Azure VM on push to `main`, builds `spam/dockerfile`, pushes to
ACR, applies `kubernetes/spam-production.yaml` (PVC + Deployment + Service) and
`kubernetes/charles_ingress.yaml` (the shared Traefik IngressRoute, which now
has a `spam.charlesragone.com` route), then rolls out. Uses the existing
ACR/Azure secrets; no new secrets needed.

One-time: in Cloudflare, add a proxied `A` record `spam` → the Azure VM public
IP (same as the other subdomains). The `charlesragone-com-tls` origin cert must
cover `spam.charlesragone.com` (true if it is a wildcard, as for the other
subdomains).

The SQLite PVC is `ReadWriteOnce`, so keep `prod-spam` at one replica.
