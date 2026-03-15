#!/usr/bin/env bash
# 初回セットアップスクリプト
# 使い方: bash scripts/setup.sh

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# Homebrew 経由でインストール（macOS 専用）
brew_install() {
    local pkg="$1"
    local cmd="${2:-$1}"   # コマンド名が package 名と異なる場合に指定
    if command -v "$cmd" &>/dev/null; then
        info "$cmd: already installed"
    else
        info "Installing $pkg via Homebrew..."
        brew install "$pkg"
        info "$pkg installed"
    fi
}

info "=== MicroMart Setup ==="

# ── Homebrew 確認 ─────────────────────────────────────────────────────────────
if ! command -v brew &>/dev/null; then
    error "Homebrew not found. Install it first: https://brew.sh"
fi

# ── 必須ツールのインストール ────────────────────────────────────────────────────
info "Installing required tools..."

# docker / colima 確認
if ! command -v docker &>/dev/null; then
    error "Docker not found. Install via: brew install colima docker"
fi
info "docker: OK"

# Colima 使用時のリソース確認 (docker が応答するかで起動確認)
if command -v colima &>/dev/null; then
    if ! docker info &>/dev/null 2>&1; then
        warn "Colima is not running. Start with:"
        warn "  colima start --cpu 4 --memory 8 --vm-type vz"
        error "Please start Colima first"
    fi
    COLIMA_MEM=$(colima list --json 2>/dev/null \
        | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['memory']//1073741824)" \
        2>/dev/null || echo 0)
    if [[ "$COLIMA_MEM" -lt 6 ]]; then
        warn "Colima memory is ${COLIMA_MEM}GiB (need 6GiB+ for kind). Restart with:"
        warn "  colima stop && colima start --cpu 4 --memory 8 --vm-type vz"
        error "Insufficient Colima memory"
    fi
    info "Colima: ${COLIMA_MEM}GiB OK"
fi

# docker compose (plugin 版: docker compose / スタンドアロン版: docker-compose 両方対応)
if docker compose version &>/dev/null 2>&1; then
    info "docker compose: OK"
elif command -v docker-compose &>/dev/null; then
    info "docker-compose (standalone): OK"
else
    error "docker compose not found. Update Docker Desktop to v2.x or run: brew install docker-compose"
fi

brew_install protobuf protoc
brew_install kind
brew_install skaffold

# kubectl は Docker Desktop に同梱されている場合も多いのでスキップ可
if ! command -v kubectl &>/dev/null; then
    brew_install kubectl
else
    info "kubectl: OK"
fi

# golang-migrate
if ! command -v migrate &>/dev/null; then
    brew_install golang-migrate migrate
else
    info "migrate: OK"
fi

# ── Go バージョン確認 ─────────────────────────────────────────────────────────
if ! command -v go &>/dev/null; then
    error "Go not found. Install via: brew install go"
fi
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED="1.22"
if [[ "$(printf '%s\n' "$REQUIRED" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED" ]]; then
    error "Go $REQUIRED+ required, found $GO_VERSION"
fi
info "Go version: $GO_VERSION OK"

# ── protoc プラグイン ─────────────────────────────────────────────────────────
info "Installing protoc Go plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
info "protoc plugins installed"

# ── golangci-lint ─────────────────────────────────────────────────────────────
if ! command -v golangci-lint &>/dev/null; then
    info "Installing golangci-lint..."
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
        | sh -s -- -b "$(go env GOPATH)/bin" v1.58.2
else
    info "golangci-lint: OK"
fi

# ── kind クラスタ + local registry ───────────────────────────────────────────
info "Setting up kind cluster and local registry..."
bash scripts/kind-setup.sh

# ── .env ファイル生成 ──────────────────────────────────────────────────────────
if [[ ! -f .env ]]; then
    info "Generating .env file..."
    cat > .env <<'EOF'
# ローカル開発用環境変数
# 本番では絶対にこのファイルをコミットしないこと

JWT_SECRET=dev-only-secret-do-not-use-in-prod
EOF
    info ".env file created"
else
    info ".env already exists, skipping"
fi

info ""
info "=== Setup complete! ==="
info ""
info "Next steps:"
info "  1. make dev-infra    # インフラを起動 (Docker Compose)"
info "  2. make proto        # gRPC コードを生成"
info "  3. make migrate-up   # DB マイグレーション適用"
info "  4. make test         # テスト実行"
info ""
info "  K8s 開発フロー:"
info "  5. skaffold dev --port-forward --profile=infra-only"
