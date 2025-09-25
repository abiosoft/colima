{
  description = "Container runtimes on macOS (and Linux) with minimal setup";

  # Last revision with go_1_23
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/25.05";

  outputs = { self, nixpkgs, flake-utils }: flake-utils.lib.eachDefaultSystem
    (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages.default = import ./colima.nix { inherit pkgs; };
        devShell = import ./shell.nix { inherit pkgs; };
        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/colima";
        };
      }
    );
}
