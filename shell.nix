{ pkgs ? import <nixpkgs> { } }:
let
  inherit (pkgs) lib buildGoPackage fetchFromGitHub;

  spiff = buildGoPackage rec {
    pname = "minio-exporter";
    version = "1.6.0-beta-1";
    rev = "v${version}";

    goPackagePath = "github.com/mandelsoft/spiff";
    src = fetchFromGitHub {
      inherit rev;
      owner = "mandelsoft";
      repo = "spiff";
      sha256 = "1jrdjcywlqfracr4w26yz1hxqjbdpqawalpaf2jallsmdbbcsqi3";
    };

    goDeps = ./hack/nix/spiff/deps.nix;

    meta = with lib; {
      description = "In-domain YAML templating engine spiff++";
      homepage = "https://github.com/mandelsoft/spiffr";
      license = licenses.asl20;
      platforms = platforms.unix;
    };
  };

in pkgs.mkShell {
  nativeBuildInputs = with pkgs;
    [
      awscli
      coreutils
      curl
      docker
      git
      gnumake
      gnused
      go
      iproute
      kops
      kubectl
      kubernetes-helm
      minikube
      openvpn
      protobuf
      screen
      yaml2json
    ] ++ [ spiff ];
}
