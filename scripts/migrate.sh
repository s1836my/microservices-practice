#!/usr/bin/env bash
# DB マイグレーション実行スクリプト
# 使い方:
#   bash scripts/migrate.sh up          # 全サービスを最新に
#   bash scripts/migrate.sh up user     # user サービスのみ
#   bash scripts/migrate.sh down user   # user を1つ戻す

set -euo pipefail

DIRECTION="${1:-up}"
TARGET_SVC="${2:-all}"

# サービス → DATABASE_URL のマッピング
declare -A DB_URLS=(
    [user]="postgres://micromart:micromart_secret@localhost:5432/micromart_users?sslmode=disable"
    [product]="postgres://micromart:micromart_secret@localhost:5433/micromart_products?sslmode=disable"
    [order]="postgres://micromart:micromart_secret@localhost:5434/micromart_orders?sslmode=disable"
    [payment]="postgres://micromart:micromart_secret@localhost:5435/micromart_payments?sslmode=disable"
)

run_migrate() {
    local svc="$1"
    local url="${DB_URLS[$svc]}"
    local migration_dir="services/$svc/migrations"

    if [[ ! -d "$migration_dir" ]]; then
        echo "[migrate] SKIP: $migration_dir not found"
        return
    fi

    echo "[migrate] $svc → $DIRECTION..."
    if [[ "$DIRECTION" == "down" ]]; then
        migrate -path "$migration_dir" -database "$url" down 1
    else
        migrate -path "$migration_dir" -database "$url" up
    fi
    echo "[migrate] $svc done"
}

if [[ "$TARGET_SVC" == "all" ]]; then
    for svc in "${!DB_URLS[@]}"; do
        run_migrate "$svc"
    done
else
    run_migrate "$TARGET_SVC"
fi

echo "[migrate] All migrations complete"
