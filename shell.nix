{ pkgs ? import <nixpkgs> { } }:

pkgs.mkShell {
  # nativeBuildInputs is usually what you want -- tools you need to run
  nativeBuildInputs = with pkgs.buildPackages; [
    go_1_20
    git
    lima
    qemu
  ];
  shellHook = ''
    echo Nix Shell with $(go version)
    echo

    COLIMA_BIN="$PWD/$(make print-binary-name)"
    if [ ! -f "$COLIMA_BIN" ]; then
        echo "Run 'make' to build Colima."
        echo
    fi

    set -x
    set -x
    alias colima="$COLIMA_BIN"
    set +x
  '';
}
