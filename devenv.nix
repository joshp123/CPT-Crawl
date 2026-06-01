{ pkgs, ... }:

{
  packages = [
    pkgs.age
    pkgs.go
  ];

  scripts.cpt-crawl.exec = ''
    go run ./cmd/cpt-crawl "$@"
  '';
}
