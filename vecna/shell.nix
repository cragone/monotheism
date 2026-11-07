{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
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
}
