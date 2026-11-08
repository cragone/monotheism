{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  packages = [
    pkgs.nodejs_22
    pkgs.go
    pkgs.gopls
    pkgs.git
  ];

  shellHook = ''
    export GOPATH="$PWD/.go"
    export PATH="$GOPATH/bin:$PATH"
    export PATH="$PWD/scripts:$PATH"
    echo "vecna dev shell: run-server.sh | run-frontend.sh"
  '';
}
