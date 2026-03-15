# MicroMart 実装 TODO

> 最終更新: 2026-03-13
> 現在地: Phase 2 進行中（User Service・Product Service・API Gateway 実装済み）

---

## 現状サマリー

| サービス | 状態 | 備考 |
|---------|------|------|
| User Service | ✅ 実装済み | handler / service / repository / test 完備 |
| Product Service | ✅ 実装済み | handler / service / repository / test 完備・Transactional Outbox / Kafka Producer 実装済み |
| API Gateway | ✅ 実装済み | Gin / JWT / Circuit Breaker / Rate Limit / テスト完備 |
| Search Service | ❌ 未実装 | proto 定義あり |
| Cart Service | ❌ 未実装 | proto 定義あり |
| Order Service | ❌ 未実装 | proto / event schema あり |
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

### 3-A: Search Service

- [ ] `services/search/internal/` — Elasticsearch クライアント初期化
- [ ] インデックス管理: `products` インデックスのマッピング定義
- [ ] gRPC ハンドラー: `Search`, `GetSuggestions` (proto/search/v1/search.proto)
- [ ] Kafka Consumer: `product.events` トピックを消費して Elasticsearch を更新 (CQRS Read Side)
- [ ] 単体テスト

### 3-B: Cart Service

- [ ] `services/cart/internal/` — Redis クライアント初期化
- [ ] カートデータ構造: `cart:{user_id}` キーに Hash 形式で格納、TTL 7日
- [ ] gRPC ハンドラー: `AddItem`, `RemoveItem`, `GetCart`, `ClearCart` (proto/cart/v1/cart.proto)
- [ ] 単体テスト

---

## Phase 4: 注文・決済フロー

> ⚠️ Saga パターンの実装。Order が Orchestrator、Payment と Product が Participant。

### 4-A: Order Service (Saga Orchestrator)

- [ ] `services/order/migrations/` — orders, order_items, saga_state テーブル
- [ ] `services/order/internal/repository/` — PostgreSQL 実装
- [ ] Saga ステートマシン実装:
  ```
  CREATED → PAYMENT_PENDING → PAYMENT_COMPLETED → INVENTORY_RESERVING → COMPLETED
                                                                       → CANCELLED (補償)
  ```
- [ ] Transactional Outbox パターン: DB トランザクションと Kafka 発行の原子性確保
- [ ] Kafka Producer: `order.events` トピックへ発行
- [ ] Kafka Consumer: `payment.events`, `product.events` (在庫確保結果) を消費
- [ ] gRPC ハンドラー: `CreateOrder`, `GetOrder`, `CancelOrder`
- [ ] 統合テスト (Saga フロー全体)

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
3. Search Service      ← 商品が登録できたら検索も動かせる
4. Cart Service        ← 検索→カート→注文の流れを作る
5. Order Service       ← Saga の中核
6. Payment Service     ← Saga の参加者
7. Notification        ← ほぼ Kafka Consumer のみ、シンプル
8. オブザーバビリティ   ← 横断的、最後に整える
9. Kubernetes          ← 全サービス実装後に検証
10. 品質・ドキュメント  ← 仕上げ
```

---

## 実装時の注意事項

- **User Service を参考モデルとして使う**: ディレクトリ構成・エラーハンドリング・テストパターンが揃っている
- **proto は変更しない**: 既に定義済みの `.proto` ファイルに準拠して実装する
- **設定は `deployments/config/service.yaml` テンプレートに従う**
- **Kafka のトピック名・イベントスキーマは `docs/design/event-schema.md` に従う**
- **DB スキーマは `docs/design/database-schema.md` に従う**
