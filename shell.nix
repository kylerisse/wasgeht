{ pkgs ? import <nixpkgs> { } }:
pkgs.mkShell {
  nativeBuildInputs = with pkgs; [
    go
    gnumake
    rrdtool
    unixtools.ping
  ];
}
