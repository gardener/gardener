{ pkgs ? import <nixpkgs> { } }:
pkgs.mkShell {
  nativeBuildInputs = with pkgs; [
    go
    protobuf
    docker
    screen
    git
    kubernetes-helm
    openvpn
    coreutils
    gnused
    kubectl
    iproute
    minikube
    yaml2json
    gnumake
    kops
    awscli
  ];
}
