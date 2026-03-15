# MicroMart — マイクロサービス ECマーケットプレイス 方針資料

> **最終更新**: 2026-03-12
> **ステータス**: 設計フェーズ

---

## 1. プロジェクト概要

### 1.1 目的

ポートフォリオとして提示可能な、本格的なマイクロサービスアーキテクチャのECマーケットプレイスを構築する。
複数の出品者が商品を販売し、ユーザーが購入・レビューできるプラットフォームを、
モダンなクラウドネイティブ技術スタックで実装する。

### 1.2 スコープ

| 項目 | 方針 |
|---|---|
| バックエンド | Go によるマイクロサービス群 |
| フロントエンド | **対象外**（将来的に追加可能な設計とする） |
| インフラ | Docker + Kubernetes |
| ドキュメント | 設計書・API定義・アーキテクチャ図を全て管理 |

---

## 2. アーキテクチャ概要

### 2.1 全体構成図

```
                    ┌──────────────────────────┐
                    │       Client (REST)       │
                    └────────────┬─────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │       API Gateway         │
                    │    (認証・ルーティング)      │
                    └─┬──┬──┬──┬──┬──┬──┬──────┘
                      │  │  │  │  │  │  │
         ┌────────────┘  │  │  │  │  │  └────────────┐
         ▼            ▼  ▼  ▼  ▼  ▼               ▼
    ┌─────────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌──────┐
    │  User   │ │Prod-│ │Sear-│ │Cart │ │Order│ │Notif-│
    │ Service │ │uct  │ │ch   │ │Svc  │ │ Svc │ │ication│
    └────┬────┘ └──┬──┘ └──┬──┘ └──┬──┘ └──┬──┘ └──┬───┘
         │         │       │       │       │       │
         ▼         ▼       ▼       ▼       ▼       │
    ┌────────┐ ┌────────┐ ┌───┐ ┌─────┐ ┌────────┐ │
    │PostgreS│ │PostgreS│ │ ES│ │Redis│ │PostgreS│ │
    │   QL   │ │   QL   │ │   │ │     │ │   QL   │ │
    └────────┘ └────────┘ └───┘ └─────┘ └────────┘ │
                                    │               │
                    ┌───────────────┘               │
                    │         Message Broker         │
                    │           (Kafka)              │
                    └────────────────────────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │     Payment Service       │
                    │      + PostgreSQL         │
                    └──────────────────────────┘
```

### 2.2 サービス一覧

| # | サービス名 | 責務 | データストア | 通信方式 |
|---|---|---|---|---|
| 1 | **API Gateway** | ルーティング、認証検証、レートリミット | — | REST (受信) → gRPC (内部転送) |
| 2 | **User Service** | ユーザー登録・認証・プロフィール管理 | PostgreSQL | gRPC |
| 3 | **Product Service** | 商品CRUD、カテゴリ管理、在庫管理 | PostgreSQL | gRPC |
| 4 | **Search Service** | 商品の全文検索・フィルタリング | Elasticsearch | gRPC |
| 5 | **Cart Service** | カート管理（ユーザー単位） | Redis | gRPC |
| 6 | **Order Service** | 注文処理、Sagaオーケストレーション | PostgreSQL | gRPC + Kafka |
| 7 | **Payment Service** | 決済処理（模擬） | PostgreSQL | gRPC + Kafka |
| 8 | **Notification Service** | 通知配信（メール模擬・WebSocket） | — | Kafka (受信) |

---

## 3. 技術スタック

### 3.1 バックエンド

| カテゴリ | 技術 | 選定理由 |
|---|---|---|
| **言語** | Go 1.22+ | 高パフォーマンス、並行処理、マイクロサービスとの高い親和性 |
| **HTTPフレームワーク** | Gin | Go最大のコミュニティ、豊富なミドルウェア、高速なルーティング |
| **サービス間通信（同期）** | gRPC + Protocol Buffers | 型安全、高速、コード自動生成 |
| **サービス間通信（非同期）** | Apache Kafka | 高スループット、イベント駆動アーキテクチャの実現 |
| **ORM** | GORM or sqlc | DB操作の抽象化（sqlcは型安全なSQLコード生成） |
| **認証** | JWT (JSON Web Token) | ステートレスな認証、マイクロサービスに適合 |
| **バリデーション** | go-playground/validator | 構造体ベースのバリデーション |
| **設定管理** | Viper | 環境変数・設定ファイルの統一管理 |

### 3.2 データストア

| 技術 | 用途 | 採用サービス |
|---|---|---|
| **PostgreSQL 16** | リレーショナルデータ永続化 | User, Product, Order, Payment |
| **Redis 7** | キャッシュ、セッション、カートデータ | Cart, (各サービスのキャッシュ) |
| **Elasticsearch 8** | 全文検索エンジン | Search |

### 3.3 インフラ・DevOps

| カテゴリ | 技術 | 説明 |
|---|---|---|
| **コンテナ** | Docker | 各サービスのコンテナ化 |
| **ローカル開発** | Docker Compose | ローカルでの全サービス一括起動 |
| **オーケストレーション** | Kubernetes (kind / Minikube) | 本番想定のデプロイメント |
| **CI/CD** | GitHub Actions | 自動テスト・ビルド・デプロイ |
| **監視** | Prometheus + Grafana | メトリクス収集・可視化 |
| **分散トレーシング** | Jaeger (OpenTelemetry) | リクエストの追跡・ボトルネック特定 |
| **ログ集約** | ELK Stack (Elasticsearch + Logstash + Kibana) | 構造化ログの集約・分析 |
| **DBマイグレーション** | golang-migrate | スキーマバージョン管理 |

---

## 4. アーキテクチャパターン・設計原則

### 4.1 採用パターン

| パターン | 適用箇所 | 概要 |
|---|---|---|
| **API Gateway** | Gateway Service | 全リクエストの単一エントリポイント。認証・ルーティング・レートリミットを集約 |
| **Database per Service** | 全サービス | 各サービスが独自のDBを所有し、他サービスのDBへ直接アクセスしない |
| **Saga パターン** | Order ↔ Payment ↔ Product | 分散トランザクションをイベント駆動で整合性を確保（注文→決済→在庫確保） |
| **CQRS** | Product ↔ Search | 書き込み（Product Service / PostgreSQL）と読み取り（Search Service / Elasticsearch）を分離 |
| **Circuit Breaker** | サービス間通信 | 障害の連鎖を防止。Sony の gobreaker を採用 |
| **Event-Driven Architecture** | Kafka経由の非同期通信 | サービス間の疎結合を実現。注文イベント・在庫変更イベント等を発行 |
| **Health Check** | 全サービス | Kubernetes の Liveness / Readiness Probe に対応 |
| **Graceful Shutdown** | 全サービス | シグナルハンドリングによる安全な停止処理 |
| **Structured Logging** | 全サービス | `log/slog` による構造化ログ出力 |

### 4.2 設計原則

- **単一責任の原則**: 1サービス = 1つのビジネスドメイン
- **疎結合・高凝集**: サービス間はAPIまたはイベントのみで通信
- **障害分離**: 1サービスの障害が全体に波及しない設計
- **Twelve-Factor App**: 環境変数による設定、ステートレス設計
- **API First**: OpenAPI / Protocol Buffers による定義駆動開発

---

## 5. ドキュメント管理方針

### 5.1 管理対象ドキュメント

本プロジェクトでは以下のドキュメントを `docs/` 配下で一元管理する。

```
docs/
├── design/
│   ├── project-overview.md          ← 本資料（方針資料）
│   ├── architecture.md              ← アーキテクチャ詳細設計
│   ├── database-schema.md           ← 全サービスのDBスキーマ定義
│   └── event-schema.md              ← Kafkaイベントスキーマ定義
├── api/
│   ├── openapi/
│   │   └── gateway.yaml             ← API Gateway の OpenAPI 3.0 定義
│   └── proto/                       ← gRPC の .proto ファイル（参照リンク）
├── adr/                             ← Architecture Decision Records
│   ├── 001-use-gin-framework.md
│   ├── 002-grpc-for-internal.md
│   └── ...
├── runbook/
│   ├── local-development.md         ← ローカル開発手順
│   ├── deployment.md                ← K8sデプロイ手順
│   └── troubleshooting.md           ← トラブルシューティング
└── diagrams/
    ├── system-architecture.drawio   ← 全体アーキテクチャ図
    ├── sequence-order-flow.drawio   ← 注文フローシーケンス図
    └── er-diagrams/                 ← ER図（サービスごと）
```

### 5.2 API定義方針

| 通信種別 | 定義形式 | ファイル配置 |
|---|---|---|
| クライアント → Gateway | **OpenAPI 3.0** (YAML) | `docs/api/openapi/` |
| サービス間（同期） | **Protocol Buffers** (.proto) | `proto/` (リポジトリルート) |
| サービス間（非同期） | **Kafka イベントスキーマ** (JSON Schema / Avro) | `docs/design/event-schema.md` |

### 5.3 ADR (Architecture Decision Records)

重要な技術的意思決定は ADR として記録する。

**フォーマット:**
```markdown
# ADR-XXX: タイトル

## ステータス
Accepted / Proposed / Deprecated

## コンテキスト
なぜこの決定が必要か

## 決定
何を決定したか

## 結果
この決定によるメリット・デメリット・影響
```

---

## 6. ディレクトリ構成

```
microservice/
├── services/
│   ├── gateway/                     # API Gateway
│   │   ├── cmd/                     # エントリポイント
│   │   │   └── main.go
│   │   ├── internal/
│   │   │   ├── handler/             # Gin ハンドラー
│   │   │   ├── middleware/          # 認証・ログ・レートリミット
│   │   │   ├── client/             # 各サービスの gRPC クライアント
│   │   │   └── config/             # 設定
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── go.sum
│   ├── user/                        # User Service（同様の構成）
│   │   ├── cmd/
│   │   ├── internal/
│   │   │   ├── handler/            # gRPC ハンドラー
│   │   │   ├── service/            # ビジネスロジック
│   │   │   ├── repository/         # データアクセス層
│   │   │   ├── model/              # ドメインモデル
│   │   │   └── config/
│   │   ├── migrations/             # DBマイグレーション
│   │   ├── Dockerfile
│   │   ├── go.mod
│   │   └── go.sum
│   ├── product/
│   ├── search/
│   ├── cart/
│   ├── order/
│   ├── payment/
│   └── notification/
├── proto/                           # Protocol Buffers 定義
│   ├── user/
│   │   └── v1/
│   │       └── user.proto
│   ├── product/
│   │   └── v1/
│   │       └── product.proto
│   ├── order/
│   │   └── v1/
│   │       └── order.proto
│   └── ...
├── pkg/                             # 共通ライブラリ
│   ├── logger/                      # 構造化ログ (slog)
│   ├── middleware/                  # 共通ミドルウェア
│   ├── errors/                     # 共通エラー型
│   ├── health/                     # ヘルスチェック
│   └── tracer/                     # OpenTelemetry 初期化
├── deployments/
│   ├── docker-compose.yml           # ローカル開発用
│   ├── docker-compose.infra.yml     # インフラのみ起動用
│   └── k8s/
│       ├── namespace.yaml
│       ├── gateway/
│       │   ├── deployment.yaml
│       │   ├── service.yaml
│       │   └── ingress.yaml
│       ├── user/
│       ├── product/
│       ├── ...
│       └── monitoring/
│           ├── prometheus/
│           └── grafana/
├── scripts/
│   ├── setup.sh                     # 初期セットアップ
│   ├── generate-proto.sh            # proto コード生成
│   └── migrate.sh                   # DBマイグレーション実行
├── docs/                            # ドキュメント（§5参照）
├── .github/
│   └── workflows/
│       ├── ci.yml                   # テスト・Lint
│       └── cd.yml                   # ビルド・デプロイ
├── Makefile                         # 共通タスクランナー
└── README.md
```

---

## 7. 開発フェーズ

### Phase 1: 基盤構築 🏗️

- リポジトリ・ディレクトリ構成のセットアップ
- 共通パッケージ (`pkg/`) の実装
- Docker Compose によるインフラ（PostgreSQL, Redis, Kafka, Elasticsearch）構築
- Protocol Buffers 定義とコード生成パイプライン構築
- CI/CD パイプライン（GitHub Actions）のセットアップ

### Phase 2: コアサービス実装 🔧

- **User Service**: 登録、ログイン、JWT発行、プロフィール管理
- **Product Service**: 商品CRUD、カテゴリ、在庫管理
- **API Gateway**: ルーティング、JWT検証、gRPC転送

### Phase 3: 商品検索・カート 🔍🛒

- **Search Service**: Elasticsearch連携、商品インデックス、全文検索API
- **Cart Service**: Redis によるカート管理

### Phase 4: 注文・決済フロー 💳

- **Order Service**: 注文作成、Sagaオーケストレーター
- **Payment Service**: 決済処理（模擬）、Saga参加者
- Kafka によるイベント駆動連携

### Phase 5: 通知・オブザーバビリティ 📡📊

- **Notification Service**: Kafka消費、通知処理
- Prometheus + Grafana によるメトリクス監視
- Jaeger による分散トレーシング
- ELK によるログ集約

### Phase 6: Kubernetes デプロイ ☸️

- 全サービスの K8s マニフェスト作成
- Ingress / Service / Deployment 設定
- HPA (Horizontal Pod Autoscaler) 設定
- kind or Minikube でのデプロイ検証

### Phase 7: 品質向上・ドキュメント整備 📝

- 単体テスト・統合テストの拡充
- API ドキュメント（OpenAPI / proto ドキュメント生成）の最終整備
- README・Runbook の充実
- アーキテクチャ図の最終化

---

## 8. 非機能要件

| 項目 | 方針 |
|---|---|
| **可用性** | Circuit Breaker、リトライ、Graceful Shutdown で障害耐性を確保 |
| **スケーラビリティ** | K8s HPA によるオートスケール対応 |
| **セキュリティ** | JWT認証、HTTPS（Ingress TLS）、入力バリデーション |
| **監視** | メトリクス（Prometheus）、トレース（Jaeger）、ログ（ELK） |
| **テスト** | 単体テスト、統合テスト、E2Eテスト（サービス間連携） |
| **パフォーマンス** | gRPC による高速通信、Redis キャッシュ |

---

## 9. 今後のTODO

- [ ] Phase 1: 基盤構築の着手
- [ ] 各サービスの詳細設計書作成
- [ ] Protocol Buffers 定義の策定
- [ ] OpenAPI 定義の策定
- [ ] DBスキーマ設計
- [ ] Kafkaイベントスキーマ設計
