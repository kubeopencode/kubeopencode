{
  description = "Development environment for KubeOpenCode";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: f (import nixpkgs {
        inherit system;
        config = { allowUnfree = true; };
      }));
    in
    {
      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          name = "kubeopencode-dev";

          packages = with pkgs; [
            # Go compiler and runtime
            go
            # Linter
            golangci-lint
            # Kubernetes & container tooling
            kubectl
            kubernetes-helm
            kind
            kustomize
            docker-client
            # Build tools
            gnumake
            git
            # Frontend & scripts
            nodejs_22
            pnpm
            corepack
          ];

          shellHook = ''
            export REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
            export GOPATH="$REPO_ROOT/.go"
            export PATH="$REPO_ROOT/bin:$GOPATH/bin:$PATH"
            mkdir -p "$REPO_ROOT/bin" "$GOPATH/bin"
            if [ ! -f "$REPO_ROOT/ui/dist/index.html" ]; then
              mkdir -p "$REPO_ROOT/ui/dist"
              cat << 'EOF' > "$REPO_ROOT/ui/dist/index.html"
<!DOCTYPE html>
<html>
<head><title>KubeOpenCode</title></head>
<body><div id="root">Development build placeholder. Run 'make ui-build' to compile full UI.</div></body>
</html>
EOF
            fi

            echo "=========================================================="
            echo "🚀 KubeOpenCode dev environment ready (via nix develop)!"
            echo "   • Go: $(go version 2>/dev/null || echo 'available')"
            echo "   • Make: $(make --version | head -n1)"
            echo "   • Kubectl: $(kubectl version --client 2>/dev/null | head -n1 || echo 'available')"
            echo "   • Helm: $(helm version --short 2>/dev/null || echo 'available')"
            echo "   • Kind: $(kind version 2>/dev/null || echo 'available')"
            echo "=========================================================="
          '';
        };
      });
    };
}
