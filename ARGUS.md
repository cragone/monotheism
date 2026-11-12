# Argus — log/metrics monitoring for both clusters

[super-argus](https://github.com/cragone/super-argus) is a separate, self-hosted
logging/metrics/alerting server (not part of this repo). It runs on the bw-server — the same
home-LAN machine that hosts vecna (see `VECNA.md`), reached the same way: locally via
`ssh-local-box.sh`, and publicly through that host's existing Cloudflare Tunnel, now also
routing `argus.charlesragone.com`. It is **not** deployed into either Kubernetes cluster
itself; it's a plain systemd service on the host, installed via super-argus's own NixOS
module (`nix/module.nix`, imported directly in `/etc/nixos/configuration.nix` from a local
clone at `/home/charlie/super-argus` — that host uses classic, non-flake `configuration.nix`,
so the module's `{ argusSrc }:` function form is imported inline rather than via flake
inputs). Secrets (`ARGUS_JWT_SECRET`, admin email/password, each agent's ingest API key) live
in `/etc/super-argus/server-env` and `/etc/super-argus/agent-env` on that host, never
committed here.

Its agent (`argus-agent`) ships logs from **both** clusters this repo deploys to:

| `cluster` label | Where the agent runs | Covers |
|---|---|---|
| `bw-server` | Host-level systemd on the bw-server itself (same NixOS module, `agent.mode = "kubernetes"`, reading `/var/log/pods` directly since that host is the k3s node) | vecna's single-node k3s cluster |
| `azure` | A DaemonSet inside the Azure cluster (`kubernetes/argus-agent-production.yaml`, namespace `argus`) | the Azure VM cluster that hosts everything else |

## Service whitelist (`allowed_labels`)

Each agent's ingest API key is minted with `allowed_labels.service` restricted to the real
Deployment names that project actually runs — a guard against a misconfigured shipper
silently creating junk streams, not a security boundary (see super-argus's own README on
`allowed_labels`). **Keep both lists below in sync with the real Deployment names** — add a
row here (and to the matching key's `allowed_labels`, re-minted or `PATCH`-updated) whenever
a new `kubernetes/*-production.yaml` Deployment is added to either cluster.

**`bw-server` key** — scoped to:
| source dir | Deployment (→ `service` label) |
|---|---|
| `vecna/` | `prod-vecna` |

**`azure` key** — scoped to:
| source dir | Deployment (→ `service` label) |
|---|---|
| `lorena/client` | `prod-lorena-services` |
| `server/` (lorena API) | `prod-lorena-api` |
| `lorena-art` | `prod-lorena-art` |
| `charles_ragone/` | `prod-charles-services` |
| `honeypot/` | `prod-honeypot` |
| `evo/client` | `prod-evo-client` |
| `evo/server` | `prod-evo-server` |
| `evo/tribe` | `prod-evo-tribe` |
| `leo/` | `prod-leo-services` |
| `biology/` (bio/chem tutoring site, deployed by `production_bio_chem.yml`) | `prod-eileen-services` |

k3s/Kubernetes platform pods (`traefik`, `coredns`, `metrics-server`, etc.) are deliberately
**not** whitelisted — only this project's own services are. Platform-pod lines are rejected,
not silently dropped, which is itself a signal the whitelist is working (see Verification
below); add them to the relevant list above if broader coverage is ever wanted.

## Azure DaemonSet rollout

`.github/workflows/argus_agent_rollout.yml` — `workflow_dispatch`-only (the agent image
changes rarely, and its source lives in the super-argus repo, not here, so there's no path in
this repo that should auto-trigger it on push). It SSHes into the Azure VM with the same
`AZURE_VM_IP`/`AZURE_PEM_PRIVATE_KEY`/`ACR_*` secrets every other `production_*.yml` workflow
already uses, pulls `super-argus` there, builds/pushes `agent/Dockerfile`, and applies
`kubernetes/argus-agent-production.yaml`.

One-time prerequisite: mint the `azure-agent` ingest key (see the whitelist above) from the
running Argus server, then add it as the **`ARGUS_AGENT_API_KEY`** secret in this repo's
`Production` environment. After that:
```sh
gh workflow run argus_agent_rollout.yml
```

## Verification

- `curl -s https://argus.charlesragone.com/healthz` → `200`.
- Sign in to the web UI; **Services** shows exactly `prod-vecna` labelled `cluster=bw-server`,
  and all ten Azure app Deployments from the table above labelled `cluster=azure` (each with a
  live/down badge — the Azure DaemonSet has in-cluster API access for liveness; the bw-server's
  host-level agent does not, so vecna shows logs but no liveness badge, which is expected).
- `{cluster="azure"}` and `{cluster="bw-server"}` in Explore both return log lines.
