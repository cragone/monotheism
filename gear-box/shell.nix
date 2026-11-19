{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  packages = [
    pkgs.nodejs_22
    pkgs.go
    pkgs.gopls
    pkgs.git
    pkgs.gh
    pkgs.piper-tts
  ];

  shellHook = ''
    export GOPATH="$PWD/.go"
    export PATH="$GOPATH/bin:$PATH"
    export PATH="$PWD/scripts:$PATH"
    echo "gear-box dev shell: run-server.sh | run-frontend.sh"
  '';
}
