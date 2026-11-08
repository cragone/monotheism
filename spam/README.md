# spam

Threads-based social site at **spam.charlesragone.com**.

- `server/` — Go API: Gin (HTTP), GORM (queries), Goose (SQL migrations in `server/migrations`,
  embedded and applied on startup), SQLite on a mounted volume. Also serves the exported web app.
- `app/` — React Native (Expo + Expo Router). One codebase for iOS, Android and web.

Auth is an opaque bearer token (stored in SecureStore on native, localStorage on web); the server
only keeps its SHA-256 hash.

## Run locally

```bash
# API on :8080
cd server && go run ./cmd/server

# app (separate terminal). Web talks to the same origin, so export it and let Go serve it:
cd app && npm install && npx expo export --platform web
cd ../server && WEB_DIR=../app/dist go run ./cmd/server      # http://localhost:8080

# or iterate on native/web with Expo, pointing at the local API
cd app && EXPO_PUBLIC_API_URL=http://<your-lan-ip>:8080 npx expo start
```

## Tests

```bash
cd server && go test ./...
cd app && npm run typecheck
```

## Deploy

`.github/workflows/production_spam.yml` builds `spam/dockerfile`, applies
`kubernetes/spam-production.yaml` and the Traefik routes in `kubernetes/charles_ingress.yaml`.
SQLite lives on a single ReadWriteOnce volume, so the Deployment stays at one replica.
