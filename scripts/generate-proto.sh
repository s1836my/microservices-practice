#!/usr/bin/env bash
# proto ファイルから Go コードを生成
# 使い方: bash scripts/generate-proto.sh

set -euo pipefail

# protoc-gen-go / protoc-gen-go-grpc は $GOPATH/bin にインストールされる
export PATH="$(go env GOPATH)/bin:$PATH"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="$REPO_ROOT/proto"
OUT_BASE="$REPO_ROOT"   # サービスの go.mod ルートに gen/ を生成

SERVICES=(user product order cart search payment)

echo "[proto] Starting code generation..."

for svc in "${SERVICES[@]}"; do
    PROTO_FILE="$PROTO_DIR/$svc/v1/$svc.proto"

    if [[ ! -f "$PROTO_FILE" ]]; then
        echo "[proto] SKIP: $PROTO_FILE not found"
        continue
    fi

    # 出力先: services/{svc}/gen/proto/{svc}/v1/
    OUT_DIR="$OUT_BASE/services/$svc/gen/proto/$svc/v1"
    mkdir -p "$OUT_DIR"

    echo "[proto] Generating: $svc..."
    protoc \
        --proto_path="$PROTO_DIR" \
        --go_out="$OUT_BASE" \
        --go_opt=module=github.com/yourorg/micromart \
        --go-grpc_out="$OUT_BASE" \
        --go-grpc_opt=module=github.com/yourorg/micromart \
        "$PROTO_FILE"

    echo "[proto] Done: $svc → $OUT_DIR"
done

echo "[proto] All done!"
