let
  pkgs = import <nixpkgs> { };
in
let
  # override Lima to remove wrapper for qemu
  # https://github.com/NixOS/nixpkgs/blob/f2537a505d45c31fe5d9c27ea9829b6f4c4e6ac5/pkgs/applications/virtualization/lima/default.nix#L35
  lima = pkgs.lima.overrideAttrs (old: {
    installPhase = ''
      runHook preInstall
      mkdir -p $out
      cp -r _output/* $out
      runHook postInstall
    '';
  });
in
pkgs.buildGo118Module rec {
  name = "colima";
  pname = "colima";
  src = ./.;
  nativeBuildInputs = with pkgs; [ installShellFiles makeWrapper git coreutils ];
  vendorSha256 = "sha256-jDzDwK7qA9lKP8CfkKzfooPDrHuHI4OpiLXmX9vOpOg=";
  preConfigure = ''
    ldflags="-X github.com/abiosoft/colima/config.appVersion=$(git describe --tags --always)
              -X github.com/abiosoft/colima/config.revision=$(git rev-parse HEAD)"
  '';
  postInstall = ''
    wrapProgram $out/bin/colima \
      --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.qemu lima ]}
    installShellCompletion --cmd colima \
      --bash <($out/bin/colima completion bash) \
      --fish <($out/bin/colima completion fish) \
      --zsh <($out/bin/colima completion zsh)
  '';
}
