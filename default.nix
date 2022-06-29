with import <nixpkgs> { };

buildGo118Module rec {
  name = "colima";
  pname = "colima";
  src = ./.;
  nativeBuildInputs = [ installShellFiles makeWrapper git ];
  vendorSha256 = "sha256-jDzDwK7qA9lKP8CfkKzfooPDrHuHI4OpiLXmX9vOpOg=";
  preConfigure = ''
    ldflags="-X github.com/abiosoft/colima/config.appVersion=$(git describe --tags --always)
              -X github.com/abiosoft/colima/config.revision=$(git rev-parse HEAD)"
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
