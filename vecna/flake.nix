{
  description = "vecna dev shell";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      forAllSystems = nixpkgs.lib.genAttrs [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
    in
    {
      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            packages = [
              pkgs.nodejs_22
              pkgs.go
              pkgs.gopls
              pkgs.mosquitto
              pkgs.python312
              pkgs.openssl
              pkgs.git
            ];

            shellHook = ''
              export GOPATH="$PWD/.go"
              export PATH="$GOPATH/bin:$PATH"
              export PATH="$PWD/scripts:$PATH"
              echo "vecna dev shell: run-mqtt.sh | setup-cync-lan.sh | run-cync-lan.sh | run-server.sh | run-frontend.sh"
            '';
          };
        });
    };
}
