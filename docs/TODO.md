# MicroMart 実装 TODO

> 最終更新: 2026-03-16
> 現在地: Phase 4 進行中（Order Service 最小実装済み、次は Payment Service と Saga 連携）

---

## 現状サマリー

| サービス | 状態 | 備考 |
|---------|------|------|
| User Service | ✅ 実装済み | handler / service / repository / test 完備 |
| Product Service | ✅ 実装済み | handler / service / repository / test 完備・Transactional Outbox / Kafka Producer 実装済み |
| API Gateway | ✅ 実装済み | Gin / JWT / Circuit Breaker / Rate Limit / テスト完備 |
| Search Service | ✅ 実装済み | gRPC / Elasticsearch / Kafka Consumer / test 実装済み |
| Cart Service | ✅ 実装済み | gRPC / Redis / health check / test 実装済み |
| Order Service | 🟡 最小実装済み | gRPC / PostgreSQL / Transactional Outbox / 単体テスト実装済み、Saga Consumer は未実装 |
| Payment Service | ❌ 未実装 | proto / event schema あり |
| Notification Service | ❌ 未実装 | event schema あり |

---

## Phase 2: コアサービス実装（次にやること）

### 2-A: Product Service ✅ 完了

User Service を参考に同じ構成で実装した。

- [x] `services/product/internal/model/` — Product, Category, Inventory, OutboxEvent モデル
- [x] `services/product/migrations/` — 4マイグレーション (categories / products / inventories / product_outbox)
- [x] `services/product/internal/repository/` — PostgreSQL 実装 (products, inventories, product_outbox テーブル)
- [x] `services/product/internal/service/` — 商品 CRUD、在庫管理ロジック
- [x] `services/product/internal/handler/` — gRPC ハンドラー (proto/product/v1/product.proto に準拠)
- [x] `services/product/internal/config/` — Viper 設定読み込み
- [x] `services/product/cmd/main.go` — サーバー起動 (gRPC: 50052, HTTP: 8081)
- [x] Kafka Producer: 商品作成/更新/削除イベントを `product.events` トピックに発行 (Transactional Outbox パターン)
- [x] 単体テスト (service 74.5% / handler 72.4% / validator 85.7%)

**実装メモ:**
- `OutboxRelay` が5秒ごとに `product_outbox` をポーリングして Kafka に発行
- `KAFKA_ENABLED=false` で NoopPublisher に切り替え可能（開発環境対応）
- `ReserveInventory` は `SELECT FOR UPDATE` で排他ロックして原子的に在庫予約

**参照ドキュメント:**
- `docs/design/database-schema.md` — products / categories / inventory テーブル定義
- `proto/product/v1/product.proto` — gRPC インターフェース
- `docs/design/event-schema.md` — Kafka イベントスキーマ

---

### 2-B: API Gateway ✅ 完了

- [x] `services/gateway/internal/config/` — Viper 設定（各サービスの gRPC エンドポイント）
- [x] `services/gateway/internal/client/` — User / Product / Search / Cart / Order の gRPC クライアント (Circuit Breaker 組み込み)
- [x] `services/gateway/internal/middleware/` — JWT 認証ミドルウェア、レートリミット (golang.org/x/time/rate)、構造化ログ
- [x] `services/gateway/internal/handler/` — Gin ルーターと各サービスへの転送ハンドラー
- [x] `services/gateway/cmd/main.go` — Gin サーバー起動
- [ ] 統合テスト — 認証フロー、ルーティング確認（Search / Cart / Order 実装後）

**実装メモ:**
- HTTP REST (8080) → gRPC 内部サービス変換
- JWT ミドルウェア: Bearer トークン検証 → user_id / role を Gin context に格納
- Circuit Breaker: `gobreaker` (MaxRequests: 5, Timeout: 30s, Failure Ratio: 50%)
- Rate Limiter: IP ごとのトークンバケット (`golang.org/x/time/rate`、デフォルト 100 RPS / burst 200)
- Search / Cart / Order サービス未実装でもサービス起動可能（Circuit Breaker が接続エラーを吸収）
- 単体テスト (middleware 100% / auth_handler 100% / response 92.9%)

**参照ドキュメント:**
- `docs/api/openapi/gateway.yaml` — REST API 定義（全エンドポイント）
- `docs/adr/001-use-gin-framework.md`
- `docs/adr/002-grpc-for-internal.md`

---

## Phase 3: 検索・カート

### 3-A: Search Service ✅ 完了

- [x] `services/search/internal/config/` — Viper 設定読み込み
- [x] `services/search/internal/repository/` — Elasticsearch HTTP クライアント実装、`products` インデックス管理
- [x] `services/search/internal/service/` — 検索ロジック、`product.events` Consumer、イベント反映
- [x] `services/search/internal/handler/` — gRPC ハンドラー (`SearchProducts`)
- [x] `services/search/cmd/main.go` — サーバー起動 (gRPC: 50053, HTTP: 8082)
- [x] 単体テスト (service / handler / repository)

**実装メモ:**
- Product Service の outbox が発行する `product.events` を消費して Elasticsearch を更新
- Elasticsearch 連携は HTTP API ベース。起動時に `products` インデックスの存在確認と作成を実施
- 現時点では service 実装を優先し、Docker Compose / Kubernetes 側の配線は後段でまとめて調整する
- カバレッジ現況: service 58.5% / repository 54.7% / handler 100% / サービス全体 42.4%

**参照ドキュメント:**
- `proto/search/v1/search.proto` — gRPC インターフェース
- `docs/design/event-schema.md` — `product.events` イベント定義
- `docs/design/architecture.md` — CQRS Read Side 方針

### 3-B: Cart Service ✅ 完了

- [x] `services/cart/internal/` — Redis クライアント初期化
- [x] カートデータ構造: `cart:{user_id}` キーに Hash 形式で格納、TTL 7日
- [x] gRPC ハンドラー: `GetCart`, `AddItem`, `UpdateItem`, `RemoveItem`, `ClearCart` (proto/cart/v1/cart.proto)
- [x] `services/cart/internal/config/` — Viper 設定読み込み
- [x] `services/cart/cmd/main.go` — サーバー起動 (gRPC: 50054, HTTP: 8083)
- [x] 単体テスト
- [ ] Product Service 連携: `AddItem` 時に `product_name` / `unit_price` を補完
- [ ] Docker Compose / Kubernetes 側の Cart Service 配線確認

**実装メモ:**
- Redis Hash で `cart:{user_id}` を管理し、更新時に TTL 7日を再設定
- service 層で数量更新・削除・合計金額 / 商品数集計を担当
- health check は Redis `PING` を利用
- カバレッジ現況: service 81.0% / repository 79.5% / handler 85.2% / model 92.9% / config 100% / サービス全体 68.0%

**参照ドキュメント:**
- `proto/cart/v1/cart.proto` — gRPC インターフェース
- `docs/design/architecture.md` — Redis キー設計

---

## Phase 4: 注文・決済フロー

> ⚠️ Saga パターンの実装。Order が Orchestrator、Payment と Product が Participant。

### 4-A: Order Service (Saga Orchestrator)

- [x] `services/order/migrations/` — orders, order_items, order_saga_state, order_outbox テーブル
- [x] `services/order/internal/repository/` — PostgreSQL 実装
- [ ] Saga ステートマシン実装:
  ```
  CREATED → PAYMENT_PENDING → PAYMENT_COMPLETED → INVENTORY_RESERVING → COMPLETED
                                                                       → CANCELLED (補償)
  ```
- [x] Transactional Outbox パターン: DB トランザクションと Kafka 発行の原子性確保
- [x] Kafka Producer: `order.events` トピックへ発行
- [ ] Kafka Consumer: `payment.events`, `product.events` (在庫確保結果) を消費
- [x] gRPC ハンドラー: `CreateOrder`, `GetOrder`, `ListOrders`, `CancelOrder`
- [ ] 統合テスト (Saga フロー全体)
- [ ] Product Service 連携: `CreateOrder` 時に `product_name` / `seller_id` / `unit_price` を補完
- [ ] `CancelOrder` の補償フローを `payment.events` / `inventory.*` と接続

**実装メモ:**
- `services/order` を追加し、`cmd / config / model / repository / service / handler / migrations` を実装
- Create/Get/List/Cancel の基本フローは動作。注文作成時に `order_outbox` へ `order.created` を保存
- `OutboxRelay` が5秒ごとに `order_outbox` をポーリングして Kafka に発行
- `KAFKA_ENABLED=false` で NoopPublisher に切り替え可能
- 現状の `proto/order/v1/order.proto` は `product_id` / `quantity` しか持たないため、商品スナップショット情報は暫定値で保持
- カバレッジ現況: handler 79.1% / service 56.9% / サービス全体 20.9%

### 4-B: Payment Service (Saga Participant)

- [ ] `services/payment/migrations/` — payments テーブル (べき等性キー付き)
- [ ] べき等性保証: `idempotency_key` による重複処理防止
- [ ] 決済処理ロジック (模擬: 常に成功 or ランダム失敗)
- [ ] Kafka Consumer: `order.payment_requested` イベント消費
- [ ] Kafka Producer: `payment.completed` / `payment.failed` 発行
- [ ] 補償トランザクション: `payment.refund_requested` 消費時に返金処理

---

## Phase 5: 通知・オブザーバビリティ

### 5-A: Notification Service

- [ ] Kafka Consumer: `order.events`, `payment.events` を消費
- [ ] 通知ロジック: ログ出力（メール送信の模擬）
- [ ] (オプション) WebSocket エンドポイント: リアルタイム通知

### 5-B: オブザーバビリティ強化

現状は OpenTelemetry のトレーシング基盤のみ。以下を追加:

- [ ] Prometheus メトリクス: 各サービスに `/metrics` エンドポイント追加 (`prometheus/client_golang`)
- [ ] Grafana ダッシュボード: `deployments/docker-compose.infra.yml` に prometheus + grafana 追加
- [ ] Jaeger 動作確認: ローカル開発環境で分散トレース可視化
- [ ] 構造化ログの Kibana 確認: `make dev-infra-full` で Kibana 起動して確認

---

## Phase 6: Kubernetes 対応

K8s マニフェストはすでに `deployments/k8s/` に存在するが、実装が追いついていないサービス分は未検証。
現時点では各 service 実装を優先し、Docker Compose / K8s / Config の調整は主要サービスが揃ってから横断で実施する。

- [ ] 全サービスの Docker イメージビルド確認 (`make build`)
- [ ] `make kind-setup` でローカル k8s クラスタ起動
- [ ] 各サービスの Deployment/Service マニフェスト動作確認
- [ ] Ingress 設定確認 (gateway → 外部公開)
- [ ] HPA 動作確認 (負荷テスト)
- [ ] ConfigMap / Secret の整備

---

## Phase 7: 品質向上・仕上げ

- [ ] 全サービスのテストカバレッジ 80%+ 達成 (`make test-coverage`)
- [ ] 統合テスト追加: Docker Compose 起動状態でのサービス間疎通テスト
- [ ] `docs/runbook/local-development.md` 作成
- [ ] `docs/runbook/deployment.md` 作成
- [ ] `docs/diagrams/` — アーキテクチャ図・シーケンス図の作成
- [ ] README.md の充実 (ポートフォリオとして見せるため)
- [ ] OpenAPI ドキュメントの最終確認 (`docs/api/openapi/gateway.yaml`)

---

## 実装優先順位まとめ

```
1. Product Service     ✅ 完了
2. API Gateway         ✅ 完了
3. Search Service      ✅ 完了
4. Cart Service        ✅ 完了
5. Order Service 基本形 ✅ 完了
6. Payment Service     ← 次の本命。Saga の参加者
7. Order Saga 連携     ← payment.events / inventory.* を消費
8. Notification        ← ほぼ Kafka Consumer のみ、シンプル
9. Docker Compose / K8s / Config
10. オブザーバビリティ  ← 横断的、最後に整える
11. 品質・ドキュメント  ← 仕上げ
```

---

## テストカバレッジ現況（2026-03-16 実測）

| サービス | 主なカバレッジ | 備考 |
|---------|---------------|------|
| User Service | service 82.2% / handler 78.4% / validator 100% | 一部パッケージは 80% 超え、サービス全体では未達 |
| Product Service | service 74.5% / handler 72.4% / validator 85.7% | repository / config / cmd 未テスト |
| Search Service | service 58.5% / repository 54.7% / handler 100% | `docs/TODO.md` 記載どおり全体はまだ低い |
| Cart Service | service 81.0% / repository 79.5% / handler 85.2% / model 92.9% / config 100% | サービス全体は 68.0% |
| Order Service | service 56.9% / handler 79.1% | repository / config / cmd 未テストで全体 20.9% |

---

## 実装時の注意事項

- **User Service を参考モデルとして使う**: ディレクトリ構成・エラーハンドリング・テストパターンが揃っている
- **proto は変更しない**: 既に定義済みの `.proto` ファイルに準拠して実装する
- **service 実装を先に揃える**: deployment の env / manifest 調整は個別最適せず、主要 service 実装後にまとめて行う
- **Kafka のトピック名・イベントスキーマは `docs/design/event-schema.md` に従う**
- **DB スキーマは `docs/design/database-schema.md` に従う**
