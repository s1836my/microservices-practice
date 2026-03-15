# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MicroMart — Go マイクロサービス ECマーケットプレイス。8つの独立サービスが gRPC (同期) と Kafka (非同期) で通信する。

## Common Commands

```bash
# インフラ起動
make dev-infra              # PostgreSQL/Redis/Kafka/Elasticsearch のみ
make dev-infra-full         # + Kibana, Kafdrop (監視UI)
make dev                    # 全サービス起動

# テスト
make test                   # 全サービス
make test-{service}         # 特定サービス (例: make test-user)
make test-coverage          # coverage.html 生成
make test-pkg               # pkg/ 共有ライブラリ

# 単一サービスで直接実行
cd services/user && go test ./... -race -count=1 -v
cd services/user && go test ./internal/service/... -run TestUserService -v

# Lint
make lint                   # 全サービス (golangci-lint)
make lint-pkg               # pkg/

# ビルド
make build                  # 全 Docker イメージ
make build-{service}        # 特定サービス

# Proto コード生成
make proto                  # proto/*.proto → services/*/gen/

# DB マイグレーション
make migrate-up             # 全サービス
make migrate-up-{service}   # 特定サービス

# Kubernetes (kind)
make kind-setup && make kind-load && make skaffold-dev
make k8s-status
make k8s-logs-{service}
```

## Architecture

### Services (8つ)

| サービス | 責務 | DB | 通信 |
|---------|------|-----|------|
| **gateway** | REST エントリポイント、JWT 認証、レートリミット | — | REST (受信) → gRPC (内部) |
| **user** | 登録・認証・JWT発行 | PostgreSQL | gRPC |
| **product** | 商品CRUD・在庫管理 | PostgreSQL | gRPC + Kafka (発行) |
| **search** | 全文検索 (CQRS Read Side) | Elasticsearch | gRPC + Kafka (消費) |
| **cart** | カート管理 (TTL 7日) | Redis | gRPC |
| **order** | 注文・Saga オーケストレーター | PostgreSQL | gRPC + Kafka |
| **payment** | 決済処理・べき等性保証 | PostgreSQL | gRPC + Kafka |
| **notification** | 通知配信 | — | Kafka (消費のみ) |

### Key Patterns

**Saga パターン** (Order → Payment → Product): Order Service がオーケストレーター。Kafka イベントで `CREATED → PAYMENT_PENDING → PAYMENT_COMPLETED → INVENTORY_RESERVING → COMPLETED/CANCELLED` のステートマシンを駆動。

**CQRS**: Product Service (Write/PostgreSQL) → Kafka → Search Service (Read/Elasticsearch)

**Circuit Breaker**: `gobreaker` (MaxRequests: 5, Timeout: 30s, Failure Ratio: 50%)

### Service Internal Structure

```
services/{service}/
├── cmd/main.go              # エントリポイント
├── internal/
│   ├── config/              # Viper 設定
│   ├── handler/             # gRPC ハンドラー
│   ├── service/             # ビジネスロジック
│   ├── repository/          # データアクセス (interface + pg実装)
│   ├── model/               # ドメインモデル
│   └── validator/           # zod 相当の入力バリデーション
├── migrations/              # golang-migrate
└── gen/                     # 自動生成 proto コード
```

`main.go` の標準的な起動順: 設定 → ロガー → OpenTelemetry → DB接続 → Repository → Service → Handler → HTTP ヘルスチェック → gRPC サーバー (Graceful Shutdown)

### Shared Library (`pkg/`)

- `grpcserver/` — gRPC サーバー起動テンプレート・インターセプター
- `health/` — `/health`, `/ready` エンドポイント
- `logger/` — 構造化ログ (log/slog)
- `tracer/` — OpenTelemetry 初期化
- `errors/` — 共通エラー型

### Infrastructure Ports

| サービス | ポート |
|---------|------|
| postgres-users | 5432 |
| postgres-products | 5433 |
| postgres-orders | 5434 |
| postgres-payments | 5435 |
| Redis | 6379 |
| Kafka | 9092 |
| Elasticsearch | 9200 |
| Kibana | 5601 |
| Kafdrop | 9000 |

## Technology Stack

- **言語**: Go 1.22
- **HTTP**: Gin (gateway のみ)
- **RPC**: gRPC + Protocol Buffers
- **メッセージング**: Apache Kafka
- **認証**: JWT (`golang-jwt/jwt`)
- **DB ドライバー**: `pgx/v5`
- **設定**: Viper
- **トレーシング**: OpenTelemetry + Jaeger
- **デプロイ**: Docker Compose (開発) / Kubernetes kind (本番想定)

## Go Workspace

`go.work` でマルチモジュール構成。`pkg/` と各 `services/*` が独立した `go.mod` を持つ。新サービス追加時は `go work use ./services/{service}` が必要。

## Development Phases

現在 **Phase 2 (コアサービス)** — User Service 実装済み。Product Service と API Gateway が次のターゲット。
