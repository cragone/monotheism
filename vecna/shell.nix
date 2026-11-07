{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  packages = [
    pkgs.nodejs_22
    pkgs.go
    pkgs.gopls
    pkgs.mosquitto
  ];

  shellHook = ''
    export GOPATH="$PWD/.go"
    export PATH="$GOPATH/bin:$PATH"
  '';
}
