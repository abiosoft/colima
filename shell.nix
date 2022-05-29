{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  # nativeBuildInputs is usually what you want -- tools you need to run
  nativeBuildInputs = with pkgs.buildPackages; [
    go_1_18
    git
    coreutils
  ];
  shellHook = ''
    echo export CGO_ENABLED=0
  '';
}
