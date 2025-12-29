{ pkgs, lib, config, inputs, ... }:

{
  packages = [ 
    pkgs.git 
    pkgs.gcc # Often needed for CGO if PocketBase uses sqlite features
  ];

  languages.go.enable = true;

  scripts.build.exec = ''
    # Ensure we are targeting ARM64 Linux
    export GOOS=linux
    export GOARCH=arm64
    
    # CGO_ENABLED=1 is usually required for PocketBase because of SQLite
    export CGO_ENABLED=1 
    
    echo "Building PocketBase for $GOARCH..."
    go build -o bin/pocketbase ./cmd 
    echo "Done! Binary located in ./bin/pocketbase"
  '';
}