# MicroMart — アーキテクチャ詳細設計

> **最終更新**: 2026-03-12
> **ステータス**: 設計フェーズ

---

## 1. サービス詳細設計

### 1.1 API Gateway

**責務**: 全クライアントリクエストの単一エントリポイント。JWT検証・ルーティング・レートリミット。

**技術**: Go 1.22, Gin, segmentio/kafka-go (不要時省略)

#### ディレクトリ構成

```
services/gateway/
├── cmd/main.go
├── internal/
│   ├── config/config.go          # Viper による設定読み込み
│   ├── handler/
│   │   ├── user.go               # /api/v1/users/* → User Service gRPC
│   │   ├── product.go            # /api/v1/products/* → Product Service gRPC
│   │   ├── search.go             # /api/v1/search/* → Search Service gRPC
│   │   ├── cart.go               # /api/v1/cart/* → Cart Service gRPC
│   │   └── order.go              # /api/v1/orders/* → Order Service gRPC
│   ├── middleware/
│   │   ├── auth.go               # JWT検証ミドルウェア
│   │   ├── ratelimit.go          # レートリミット（golang.org/x/time/rate）
│   │   ├── logger.go             # 構造化ログ (slog)
│   │   └── recovery.go           # パニックリカバリ
│   └── client/
│       ├── user_client.go        # User Service gRPC クライアント
│       ├── product_client.go     # Product Service gRPC クライアント
│       ├── search_client.go      # Search Service gRPC クライアント
│       ├── cart_client.go        # Cart Service gRPC クライアント
│       └── order_client.go       # Order Service gRPC クライアント
└── Dockerfile
```

#### ルーティング設計

```go
// Gin ルーティング構成
r := gin.New()
r.Use(middleware.Logger(), middleware.Recovery())

// ヘルスチェック (認証不要)
r.GET("/health", healthHandler)
r.GET("/ready", readyHandler)

// 認証不要 API
v1 := r.Group("/api/v1")
{
    auth := v1.Group("/auth")
    auth.POST("/register", handler.Register)
    auth.POST("/login",    handler.Login)
    auth.POST("/refresh",  handler.RefreshToken)

    products := v1.Group("/products")
    products.GET("",      handler.ListProducts)   // 商品一覧
    products.GET("/:id",  handler.GetProduct)     // 商品詳細
    products.GET("/search", handler.SearchProducts) // 全文検索
}

// 認証必要 API
protected := v1.Group("")
protected.Use(middleware.JWTAuth(cfg.JWT.Secret))
{
    protected.GET("/users/me",          handler.GetMe)
    protected.PUT("/users/me",          handler.UpdateProfile)

    protected.GET("/cart",              handler.GetCart)
    protected.POST("/cart/items",       handler.AddCartItem)
    protected.DELETE("/cart/items/:id", handler.RemoveCartItem)

    protected.POST("/orders",           handler.CreateOrder)
    protected.GET("/orders",            handler.ListOrders)
    protected.GET("/orders/:id",        handler.GetOrder)
}
```

#### JWT 検証ミドルウェア

```go
// internal/middleware/auth.go
func JWTAuth(secret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenStr := extractBearerToken(c.GetHeader("Authorization"))
        if tokenStr == "" {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
            return
        }

        claims, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(t *jwt.Token) (any, error) {
            if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
                return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
            }
            return []byte(secret), nil
        })
        if err != nil || !claims.Valid {
            c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
            return
        }

        userClaims := claims.Claims.(*UserClaims)
        c.Set("user_id",   userClaims.UserID)
        c.Set("user_role", userClaims.Role)
        c.Next()
    }
}
```

---

### 1.2 User Service

**責務**: ユーザー登録・認証・JWT発行・プロフィール管理

**技術**: Go 1.22, gRPC, PostgreSQL, golang-jwt/jwt

#### gRPC サーバー実装パターン

```go
// internal/handler/user_handler.go
type UserHandler struct {
    pb.UnimplementedUserServiceServer
    svc service.UserService
}

func (h *UserHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
    user, err := h.svc.Register(ctx, req.Email, req.Password, req.Name)
    if err != nil {
        return nil, status.Errorf(codes.AlreadyExists, "email already registered: %v", err)
    }
    return &pb.RegisterResponse{UserId: user.ID.String()}, nil
}

func (h *UserHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
    token, refreshToken, err := h.svc.Login(ctx, req.Email, req.Password)
    if err != nil {
        return nil, status.Errorf(codes.Unauthenticated, "invalid credentials")
    }
    return &pb.LoginResponse{
        AccessToken:  token,
        RefreshToken: refreshToken,
        ExpiresIn:    3600,
    }, nil
}
```

#### JWT 発行

```go
// internal/service/token.go
type UserClaims struct {
    UserID string `json:"user_id"`
    Email  string `json:"email"`
    Role   string `json:"role"`
    jwt.RegisteredClaims
}

func (s *tokenService) IssueAccessToken(user *model.User) (string, error) {
    claims := &UserClaims{
        UserID: user.ID.String(),
        Email:  user.Email,
        Role:   string(user.Role),
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "micromart-user-service",
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(s.secret))
}
```

---

### 1.3 Product Service

**責務**: 商品CRUD・カテゴリ管理・在庫管理。商品変更時は Kafka へイベント発行 (CQRS Write Side)

**技術**: Go 1.22, gRPC, PostgreSQL, segmentio/kafka-go

#### Kafka イベント発行

```go
// internal/event/producer.go
type ProductEventProducer struct {
    writer *kafka.Writer
}

func NewProductEventProducer(brokers []string) *ProductEventProducer {
    return &ProductEventProducer{
        writer: &kafka.Writer{
            Addr:                   kafka.TCP(brokers...),
            Topic:                  "product.events",
            Balancer:               &kafka.LeastBytes{},
            AllowAutoTopicCreation: true,
        },
    }
}

func (p *ProductEventProducer) PublishProductCreated(ctx context.Context, product *model.Product) error {
    event := ProductCreatedEvent{
        EventType: "product.created",
        ProductID: product.ID.String(),
        Name:      product.Name,
        Price:     product.Price,
        Stock:     product.Stock,
        CreatedAt: product.CreatedAt,
    }
    value, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("marshal product event: %w", err)
    }
    return p.writer.WriteMessages(ctx, kafka.Message{
        Key:   []byte(product.ID.String()),
        Value: value,
    })
}
```

---

### 1.4 Search Service

**責務**: Elasticsearch による商品全文検索 (CQRS Read Side)。Kafka から product.events を消費してインデックスを更新

**技術**: Go 1.22, gRPC, Elasticsearch 8, olivere/elastic または elastic/go-elasticsearch

#### Kafka 消費 + Elasticsearch インデックス更新

```go
// internal/consumer/product_consumer.go
func (c *ProductConsumer) Start(ctx context.Context) {
    r := kafka.NewReader(kafka.ReaderConfig{
        Brokers: c.brokers,
        GroupID: "search-service",
        Topic:   "product.events",
    })
    defer r.Close()

    for {
        msg, err := r.ReadMessage(ctx)
        if err != nil {
            if errors.Is(err, context.Canceled) {
                return
            }
            slog.Error("failed to read message", "error", err)
            continue
        }
        if err := c.handleMessage(ctx, msg); err != nil {
            slog.Error("failed to handle message", "error", err, "offset", msg.Offset)
        }
    }
}

func (c *ProductConsumer) handleMessage(ctx context.Context, msg kafka.Message) error {
    var base struct{ EventType string `json:"event_type"` }
    if err := json.Unmarshal(msg.Value, &base); err != nil {
        return fmt.Errorf("unmarshal base event: %w", err)
    }

    switch base.EventType {
    case "product.created", "product.updated":
        return c.indexer.UpsertProduct(ctx, msg.Value)
    case "product.deleted":
        return c.indexer.DeleteProduct(ctx, msg.Value)
    default:
        slog.Warn("unknown event type", "type", base.EventType)
    }
    return nil
}
```

---

### 1.5 Cart Service

**責務**: Redis を使ったカート管理（ユーザー単位のセッション的データ）

**技術**: Go 1.22, gRPC, Redis 7 (go-redis/redis)

#### Redis キー設計

```
cart:{user_id}        → Hash
  field: product:{product_id}
  value: JSON{"product_id":"...", "quantity":2, "price":1000, "name":"..."}

TTL: 7日間（最終操作から更新）
```

#### カート操作

```go
// internal/repository/cart_repository.go
const cartKeyPrefix = "cart:"
const cartTTL = 7 * 24 * time.Hour

func (r *cartRepository) AddItem(ctx context.Context, userID string, item *model.CartItem) error {
    key := cartKeyPrefix + userID
    field := "product:" + item.ProductID

    value, err := json.Marshal(item)
    if err != nil {
        return fmt.Errorf("marshal cart item: %w", err)
    }

    pipe := r.rdb.Pipeline()
    pipe.HSet(ctx, key, field, value)
    pipe.Expire(ctx, key, cartTTL)
    _, err = pipe.Exec(ctx)
    return err
}

func (r *cartRepository) GetCart(ctx context.Context, userID string) ([]*model.CartItem, error) {
    key := cartKeyPrefix + userID
    result, err := r.rdb.HGetAll(ctx, key).Result()
    if err != nil {
        return nil, fmt.Errorf("hgetall cart: %w", err)
    }

    items := make([]*model.CartItem, 0, len(result))
    for _, v := range result {
        var item model.CartItem
        if err := json.Unmarshal([]byte(v), &item); err != nil {
            return nil, fmt.Errorf("unmarshal cart item: %w", err)
        }
        items = append(items, &item)
    }
    return items, nil
}
```

---

### 1.6 Order Service

**責務**: 注文作成・Saga オーケストレーター。Payment・Product との分散トランザクション調整

**技術**: Go 1.22, gRPC, PostgreSQL, segmentio/kafka-go

#### Saga フロー

```
[Order Service]          [Payment Service]        [Product Service]
      │                         │                         │
      │── order.created ──────► │                         │
      │                         │── payment.completed ──► │
      │                         │   or payment.failed     │
      │                         │                         │── inventory.reserved
      │                         │                         │   or inventory.failed
      │◄── (aggregate result) ──┘─────────────────────────┘
      │
      │ → order.completed / order.cancelled
```

#### Saga ステートマシン

```go
// internal/saga/order_saga.go
type OrderState string

const (
    StateCreated            OrderState = "CREATED"
    StatePaymentPending     OrderState = "PAYMENT_PENDING"
    StatePaymentCompleted   OrderState = "PAYMENT_COMPLETED"
    StateInventoryReserving OrderState = "INVENTORY_RESERVING"
    StateCompleted          OrderState = "COMPLETED"
    StateCancelled          OrderState = "CANCELLED"
    StateCompensating       OrderState = "COMPENSATING"
)

type OrderSaga struct {
    orderRepo  repository.OrderRepository
    producer   event.OrderEventProducer
}

func (s *OrderSaga) HandlePaymentResult(ctx context.Context, event *PaymentResultEvent) error {
    order, err := s.orderRepo.FindByID(ctx, event.OrderID)
    if err != nil {
        return fmt.Errorf("find order: %w", err)
    }

    switch event.Status {
    case "completed":
        return s.transitionTo(ctx, order, StatePaymentCompleted,
            func() error { return s.producer.PublishInventoryReserve(ctx, order) })
    case "failed":
        return s.transitionTo(ctx, order, StateCancelled,
            func() error { return s.producer.PublishOrderCancelled(ctx, order, event.Reason) })
    }
    return nil
}
```

---

### 1.7 Payment Service

**責務**: 決済処理（模擬）。Saga の参加者として order.created を消費し payment.completed/failed を発行

**技術**: Go 1.22, gRPC, PostgreSQL, segmentio/kafka-go

#### べき等性保証

```go
// internal/service/payment_service.go
func (s *paymentService) ProcessPayment(ctx context.Context, orderID, idempotencyKey string, amount int64) error {
    // 同一 idempotency_key の処理済み確認
    existing, err := s.repo.FindByIdempotencyKey(ctx, idempotencyKey)
    if err == nil && existing != nil {
        slog.Info("payment already processed", "idempotency_key", idempotencyKey)
        return nil // 冪等応答
    }

    payment := &model.Payment{
        OrderID:        orderID,
        IdempotencyKey: idempotencyKey,
        Amount:         amount,
        Status:         model.PaymentStatusPending,
    }

    // 模擬決済処理（実際はカード処理API等を呼ぶ）
    if err := s.simulatePayment(payment); err != nil {
        payment.Status = model.PaymentStatusFailed
        payment.FailureReason = err.Error()
    } else {
        payment.Status = model.PaymentStatusCompleted
    }

    return s.repo.Save(ctx, payment)
}
```

---

### 1.8 Notification Service

**責務**: Kafka イベントを消費し、メール通知（模擬）・WebSocket プッシュを行う

**技術**: Go 1.22, segmentio/kafka-go

#### マルチトピック購読

```go
// internal/consumer/dispatcher.go
func NewNotificationDispatcher(brokers []string) *Dispatcher {
    topics := []string{
        "order.completed",
        "order.cancelled",
        "payment.completed",
        "payment.failed",
    }
    // 各トピックを goroutine で並行購読
    for _, topic := range topics {
        go d.consume(ctx, topic)
    }
}
```

---

## 2. 共通パターン

### 2.1 gRPC サーバー起動テンプレート

```go
// pkg/grpcserver/server.go
func Run(cfg Config, svc any, registerFn func(*grpc.Server)) error {
    lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
    if err != nil {
        return fmt.Errorf("listen: %w", err)
    }

    opts := []grpc.ServerOption{
        grpc.ChainUnaryInterceptor(
            otelgrpc.UnaryServerInterceptor(),   // OpenTelemetry トレーシング
            logging.UnaryServerInterceptor(...),  // 構造化ログ
            recovery.UnaryServerInterceptor(...), // パニックリカバリ
        ),
    }
    s := grpc.NewServer(opts...)
    registerFn(s)

    // Graceful Shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
    go func() {
        <-quit
        slog.Info("shutting down grpc server")
        s.GracefulStop()
    }()

    slog.Info("grpc server started", "port", cfg.Port)
    return s.Serve(lis)
}
```

### 2.2 Circuit Breaker (gobreaker)

```go
// pkg/circuitbreaker/breaker.go
import "github.com/sony/gobreaker"

func NewServiceBreaker(name string) *gobreaker.CircuitBreaker {
    return gobreaker.NewCircuitBreaker(gobreaker.Settings{
        Name:        name,
        MaxRequests: 5,
        Interval:    10 * time.Second,
        Timeout:     30 * time.Second,
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return counts.Requests >= 5 && failureRatio >= 0.5
        },
        OnStateChange: func(name string, from, to gobreaker.State) {
            slog.Warn("circuit breaker state changed",
                "name", name,
                "from", from.String(),
                "to", to.String())
        },
    })
}
```

### 2.3 Health Check エンドポイント

```go
// pkg/health/handler.go
type HealthResponse struct {
    Status  string            `json:"status"`
    Checks  map[string]string `json:"checks,omitempty"`
    Version string            `json:"version"`
}

func Handler(checks ...Checker) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        resp := HealthResponse{Status: "ok", Checks: make(map[string]string)}
        for _, c := range checks {
            name, err := c.Check(r.Context())
            if err != nil {
                resp.Status = "degraded"
                resp.Checks[name] = err.Error()
            } else {
                resp.Checks[name] = "ok"
            }
        }
        code := http.StatusOK
        if resp.Status != "ok" {
            code = http.StatusServiceUnavailable
        }
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(code)
        json.NewEncoder(w).Encode(resp)
    }
}
```

---

## 3. サービス間通信まとめ

| 呼び出し元 | 呼び出し先 | 方式 | 用途 |
|---|---|---|---|
| API Gateway | User Service | gRPC | 認証・プロフィール |
| API Gateway | Product Service | gRPC | 商品CRUD |
| API Gateway | Search Service | gRPC | 全文検索 |
| API Gateway | Cart Service | gRPC | カート操作 |
| API Gateway | Order Service | gRPC | 注文作成・参照 |
| Product Service | Kafka | Produce | 商品変更イベント |
| Search Service | Kafka | Consume | 商品インデックス更新 |
| Order Service | Kafka | Produce | 注文イベント (Saga) |
| Payment Service | Kafka | Consume/Produce | 決済 Saga 参加者 |
| Notification Service | Kafka | Consume | 通知配信 |

---

## 4. 設定管理 (Viper)

全サービス共通の環境変数設計:

```yaml
# deployments/config/service.yaml (テンプレート)
server:
  grpc_port: ${GRPC_PORT:-50051}
  http_port: ${HTTP_PORT:-8080}  # ヘルスチェック用

database:
  host: ${DB_HOST:-localhost}
  port: ${DB_PORT:-5432}
  name: ${DB_NAME}
  user: ${DB_USER}
  password: ${DB_PASSWORD}
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m

redis:
  addr: ${REDIS_ADDR:-localhost:6379}
  password: ${REDIS_PASSWORD:-}
  db: 0

kafka:
  brokers: ${KAFKA_BROKERS:-localhost:9092}

jwt:
  secret: ${JWT_SECRET}
  access_ttl: 1h
  refresh_ttl: 168h  # 7日

telemetry:
  jaeger_endpoint: ${JAEGER_ENDPOINT:-http://jaeger:14268/api/traces}
  service_name: ${SERVICE_NAME}
```
