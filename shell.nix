{ pkgs ? import <nixpkgs> {}}:
pkgs.mkShell {
  packages = with pkgs;[
    go
    gopls
    nixd
    python313
    graphviz
    (python313.withPackages (ps: with ps; [
      matplotlib
    ]))
  ];
}
