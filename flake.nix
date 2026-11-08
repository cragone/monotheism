{
  description = "monotheism: local dev shell for all three sites";

  inputs.nixpkgs.url = "nixpkgs";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forEachSystem = nixpkgs.lib.genAttrs systems;
    in
    {
      devShells = forEachSystem (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          # Lorena's booking API (Express + better-sqlite3) — this is the
          # piece that owns the actual sqlite database file, so it's the
          # "db" of the bunch even though there's no standalone DB server.
          runDb = pkgs.writeShellScriptBin "run-db" ''
            set -euo pipefail
            root="$(git rev-parse --show-toplevel)"
            cd "$root/server"
            [ -d node_modules ] || npm install
            export PORT="''${PORT:-4000}"
            export ADMIN_PASSWORD="''${ADMIN_PASSWORD:-change-me}"
            echo "==> lorena API + sqlite db on http://localhost:$PORT"
            exec npm run dev
          '';

          # Lorena's marketing + booking frontend. Proxies /api to run-db.
          runLorena = pkgs.writeShellScriptBin "run-lorena" ''
            set -euo pipefail
            root="$(git rev-parse --show-toplevel)"
            cd "$root/client"
            [ -d node_modules ] || npm install
            export VITE_API_PROXY_TARGET="''${VITE_API_PROXY_TARGET:-http://localhost:4000}"
            echo "==> lorena client on http://localhost:5173 (proxying /api -> $VITE_API_PROXY_TARGET)"
            exec npm run dev -- --port 5173 --strictPort
          '';

          # Charles's portal: a single Go binary in production, but for dev
          # with hot reload we run the Go API and the Vite frontend as two
          # processes, same as the vite proxy (localhost:8080) expects.
          runCharlesragone = pkgs.writeShellScriptBin "run-charlesragone" ''
            set -euo pipefail
            root="$(git rev-parse --show-toplevel)/charles_ragone"
            [ -d "$root/node_modules" ] || (cd "$root" && npm install)

            (
              cd "$root/server"
              export PORT="''${PORT:-8080}"
              export DATA_DIR="''${DATA_DIR:-./data}"
              export COOKIE_SECURE="''${COOKIE_SECURE:-false}"
              echo "==> charles-ragone API on http://localhost:$PORT"
              go run ./cmd/server
            ) &
            api_pid=$!
            trap 'kill "$api_pid" 2>/dev/null || true' EXIT

            cd "$root"
            echo "==> charles-ragone frontend on http://localhost:5174 (proxying /api -> http://localhost:''${PORT:-8080})"
            npm run dev -- --port 5174 --strictPort
          '';

          # Eileen's biology site: a static React/Vite frontend, no backend.
          runEileen = pkgs.writeShellScriptBin "run-eileen" ''
            set -euo pipefail
            root="$(git rev-parse --show-toplevel)/biology"
            cd "$root"
            [ -d node_modules ] || npm install
            echo "==> eileen (biology) frontend on http://localhost:5175"
            exec npm run dev -- --port 5175 --strictPort
          '';

          # Vecna's house control API + client, same two-process dev split
          # as charles-ragone. Requires an MQTT broker (and, for the real
          # plug, cync-lan) already reachable at MQTT_BROKER_URL — see
          # VECNA.md; the API will fail to start without one.
          runVecna = pkgs.writeShellScriptBin "run-vecna" ''
            set -euo pipefail
            root="$(git rev-parse --show-toplevel)/vecna"
            [ -d "$root/node_modules" ] || (cd "$root" && npm install)

            (
              cd "$root/server"
              export PORT="''${PORT:-8082}"
              export DATA_DIR="''${DATA_DIR:-./data}"
              export MQTT_BROKER_URL="''${MQTT_BROKER_URL:-tcp://localhost:1883}"
              echo "==> vecna API on http://localhost:$PORT"
              go run ./cmd/server
            ) &
            api_pid=$!
            trap 'kill "$api_pid" 2>/dev/null || true' EXIT

            cd "$root"
            echo "==> vecna frontend on http://localhost:5176 (proxying /api -> http://localhost:''${PORT:-8082})"
            npm run dev -- --port 5176 --strictPort
          '';

          # The evo game: three Rust services (cell/land server, tribal server, and the
          # wasm client). Ports are fixed, not configurable, because client/Trunk.toml
          # hardcodes proxy targets at 4000 (server) and 4200 (tribe).
          runEvo = pkgs.writeShellScriptBin "run-evo" ''
            set -euo pipefail
            root="$(git rev-parse --show-toplevel)/evo"

            (
              cd "$root/server"
              export PORT=4000
              export TRIBE_ADDR=127.0.0.1:4200
              echo "==> evo server (cell + land stages) on http://localhost:4000"
              cargo run
            ) &
            server_pid=$!

            (
              cd "$root/tribe"
              export PORT=4200
              echo "==> evo tribe server (tribal stage) on http://localhost:4200"
              cargo run
            ) &
            tribe_pid=$!

            trap 'kill "$server_pid" "$tribe_pid" 2>/dev/null || true' EXIT

            cd "$root/client"
            echo "==> evo client on http://localhost:8080 (trunk serve; proxies /api, /ws, /tribe)"
            exec trunk serve
          '';
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.go
              pkgs.nodejs
              pkgs.python3
              pkgs.gnumake
              pkgs.gcc
              pkgs.pkg-config
              pkgs.sqlite
              pkgs.git

              # evo (Rust + wasm): cargo/rustc from nixpkgs already bundle wasm32-unknown-unknown
              # std, lld is needed to link it, and trunk fetches its own matching wasm-bindgen
              # at build time so it doesn't need to match any nixpkgs wasm-bindgen-cli version.
              pkgs.cargo
              pkgs.rustc
              pkgs.lld
              pkgs.trunk

              runDb
              runLorena
              runCharlesragone
              runEileen
              runVecna
              runEvo
            ];

            shellHook = ''
              echo "monotheism dev shell"
              echo "  run-db             -> lorena API + sqlite   (http://localhost:4000)"
              echo "  run-lorena         -> lorena client         (http://localhost:5173)"
              echo "  run-charlesragone  -> charles API + client  (http://localhost:8080 / :5174)"
              echo "  run-eileen         -> eileen (biology) site (http://localhost:5175)"
              echo "  run-vecna          -> vecna API + client    (http://localhost:8082 / :5176)"
              echo "  run-evo            -> evo server + tribe + client (http://localhost:8080 / :4000 / :4200)"
            '';
          };
        }
      );
    };
}
