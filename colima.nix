{ pkgs ? import <nixpkgs> }:

with pkgs;

buildGoModule {
  name = "colima";
  pname = "colima";
  src = ./.;
  nativeBuildInputs = [ installShellFiles makeWrapper git ];
  vendorSha256 = "sha256-tsMQMWEkTE1NhevcqBETGWiboqL6QTepgnIo4B5Y4wQ=";
  CGO_ENABLED = 1;

  subPackages = [ "cmd/colima" ];

  # `nix-build` has .git folder but `nix build` does not, this caters for both cases
  preConfigure = ''
    export VERSION="$(git describe --tags --always || echo nix-build-at-"$(date +%s)")"
    export REVISION="$(git rev-parse HEAD || echo nix-unknown)"
    ldflags="-X github.com/abiosoft/colima/config.appVersion=$VERSION
              -X github.com/abiosoft/colima/config.revision=$REVISION"
  '';

  postInstall = ''
    wrapProgram $out/bin/colima \
      --prefix PATH : ${lib.makeBinPath [ qemu lima ]}
    installShellCompletion --cmd colima \
      --bash <($out/bin/colima completion bash) \
      --fish <($out/bin/colima completion fish) \
      --zsh <($out/bin/colima completion zsh)
  '';
}

