# ADR-003: 分散トランザクションに Saga パターン（オーケストレーション型）を採用する

## ステータス
Accepted

## コンテキスト

注文フローでは「注文作成 → 決済 → 在庫確保」という複数サービスにまたがるトランザクションが必要。
マイクロサービスでは各サービスが独立した DB を持つため、2PC (Two-Phase Commit) は実装が複雑でパフォーマンスに影響する。

検討パターン:
- **2PC (Two-Phase Commit)**: 強整合性だがパフォーマンス・可用性に問題
- **Saga (コレオグラフィー型)**: 各サービスがイベントを発行・購読。分散しすぎて追跡が困難
- **Saga (オーケストレーション型)**: Order Service が Saga の進行を中央管理

## 決定

**Saga パターン（オーケストレーション型）** を採用し、Order Service を Saga オーケストレーターとする。

採用理由:
1. **可視性**: Saga の状態が Order Service の `order_saga_state` テーブルで一元管理されるため、監視・デバッグが容易
2. **複雑さの局所化**: 補償トランザクションのロジックが Order Service に集約される
3. **スケーラビリティ**: 各参加者 (Payment, Product) は独立したワーカーとして動作できる

## 結果

### メリット
- 障害時の状態が明確（DB から現在ステップを確認可能）
- 補償トランザクション（在庫解放・決済返金）の実装箇所が明確

### デメリット
- Order Service への依存度が高くなる（単一障害点リスク）→ K8s HPA で冗長化して対応

### 影響
- Order Service に `order_saga_state` テーブルと状態マシンを実装
- Transactional Outbox パターンと組み合わせて AT-LEAST-ONCE 配信を保証
- Payment Service・Product Service はべき等性を確保（同一 idempotency_key の重複処理を無視）
