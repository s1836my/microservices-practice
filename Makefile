.PHONY: help dev-infra dev-infra-full dev dev-down \
        infra-down infra-clean stop-all \
        proto migrate-up migrate-down \
        build test lint \
        test-pkg lint-pkg \
        kind-setup kind-delete kind-load skaffold-run \
        k8s-apply k8s-delete \
        logs clean

# ── 設定 ────────────────────────────────────────────────────────────────────────
# docker compose (plugin v2) / docker-compose (standalone v1) を自動判別
DOCKER_COMPOSE := $(shell docker compose version &>/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")
COMPOSE      := $(DOCKER_COMPOSE) -f deployments/docker-compose.infra.yml
COMPOSE_FULL := $(DOCKER_COMPOSE) -f deployments/docker-compose.yml
KIND_CLUSTER := micromart
REGISTRY     := localhost:5001   # kind local registry

SERVICES := gateway user product search cart order payment notification

# ── ヘルプ ───────────────────────────────────────────────────────────────────────
help: ## このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}'

# ── ローカル開発 (Docker Compose) ────────────────────────────────────────────────
dev-infra: ## インフラのみ起動 (PostgreSQL / Redis / Kafka / Elasticsearch)
	$(COMPOSE) up -d
	@echo "Waiting for all services to be healthy..."
	@for svc in mm-postgres-users mm-postgres-products mm-postgres-orders mm-postgres-payments mm-redis mm-kafka mm-elasticsearch; do \
		echo -n "  $$svc ... "; \
		for i in $$(seq 1 30); do \
			status=$$(docker inspect --format='{{.State.Health.Status}}' $$svc 2>/dev/null); \
			if [ "$$status" = "healthy" ]; then echo "✅"; break; fi; \
			if [ $$i -eq 30 ]; then echo "❌ timeout"; exit 1; fi; \
			sleep 2; \
		done; \
	done
	@echo "✅ Infra ready"

dev-infra-full: ## インフラ + 監視ツール (Kibana / Kafdrop) も起動
	$(COMPOSE) --profile monitoring up -d

dev: ## 全サービス起動 (サービスコードが必要)
	$(COMPOSE_FULL) up -d

dev-down: ## 全サービス停止 (Docker Compose)
	$(COMPOSE_FULL) down

infra-down: ## インフラのみ停止 (Docker Compose)
	$(COMPOSE) down

infra-clean: ## インフラ停止 + ボリューム削除 (DB リセット)
	$(COMPOSE) down -v

stop-all: ## 全停止 (Docker Compose + kind + Colima)
	@echo "Stopping Docker Compose services..."
	-$(COMPOSE_FULL) down 2>/dev/null || true
	@echo "Stopping kind cluster..."
	-kind delete cluster --name $(KIND_CLUSTER) 2>/dev/null || true
	@echo "Stopping Colima..."
	-colima stop 2>/dev/null || true
	@echo "All stopped."

# ── Protocol Buffers ────────────────────────────────────────────────────────────
proto: ## proto ファイルから Go コードを生成
	@bash scripts/generate-proto.sh

# ── DB マイグレーション ──────────────────────────────────────────────────────────
migrate-up: ## 全サービスのマイグレーションを適用
	@bash scripts/migrate.sh up

migrate-down: ## 全サービスのマイグレーションを1つ戻す
	@bash scripts/migrate.sh down

migrate-up-%: ## 特定サービスのマイグレーション適用 (例: make migrate-up-user)
	@bash scripts/migrate.sh up $*

# ── ビルド ───────────────────────────────────────────────────────────────────────
build: ## 全サービスの Docker イメージをビルド
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		docker build -t $(REGISTRY)/micromart-$$svc:latest services/$$svc/ || exit 1; \
	done

build-%: ## 特定サービスをビルド (例: make build-user)
	docker build -t $(REGISTRY)/micromart-$*:latest services/$*/

# ── テスト ───────────────────────────────────────────────────────────────────────
test: ## 全サービスのテストを実行
	@for svc in $(SERVICES); do \
		echo "Testing $$svc..."; \
		cd services/$$svc && go test ./... -race -count=1 && cd ../..; \
	done

test-%: ## 特定サービスのテスト (例: make test-user)
	cd services/$* && go test ./... -race -count=1 -v

test-coverage: ## カバレッジレポート生成
	@for svc in $(SERVICES); do \
		cd services/$$svc && go test ./... -coverprofile=coverage.out && \
		go tool cover -html=coverage.out -o coverage.html && cd ../..; \
	done

# ── pkg/ 共有ライブラリ ──────────────────────────────────────────────────────────
test-pkg: ## pkg/ 共有ライブラリのテスト (race + coverage)
	cd pkg && go test ./... -race -count=1 -coverprofile=coverage.out
	@cd pkg && go tool cover -func=coverage.out | grep "^total:"

lint-pkg: ## pkg/ 共有ライブラリの lint
	cd pkg && golangci-lint run ./...

# ── Lint ─────────────────────────────────────────────────────────────────────────
lint: ## golangci-lint を実行
	@for svc in $(SERVICES); do \
		echo "Linting $$svc..."; \
		cd services/$$svc && golangci-lint run ./... && cd ../..; \
	done

# ── kind (ローカル K8s) ──────────────────────────────────────────────────────────
kind-setup: ## kind クラスタ + local registry を作成
	@bash scripts/kind-setup.sh

kind-delete: ## kind クラスタを削除
	kind delete cluster --name $(KIND_CLUSTER)

kind-load: build ## イメージを kind クラスタにロード
	@for svc in $(SERVICES); do \
		kind load docker-image $(REGISTRY)/micromart-$$svc:latest --name $(KIND_CLUSTER); \
	done

# ── Skaffold ─────────────────────────────────────────────────────────────────────
skaffold-dev: ## Skaffold でホットリロード開発 (kind 必要)
	skaffold dev --port-forward

skaffold-run: ## Skaffold で K8s へデプロイ
	skaffold run

# ── K8s マニフェスト ─────────────────────────────────────────────────────────────
k8s-apply: ## K8s マニフェストを適用 (インフラ → サービス順)
	kubectl apply -f deployments/k8s/namespace.yaml
	kubectl apply -f deployments/k8s/secrets.yaml
	kubectl apply -f deployments/k8s/infra/
	@echo "Waiting for infra pods..."
	kubectl wait --for=condition=ready pod -l tier=infra -n micromart --timeout=120s
	kubectl apply -f deployments/k8s/gateway/
	@for svc in user product search cart order payment notification; do \
		kubectl apply -f deployments/k8s/$$svc/; \
	done

k8s-delete: ## K8s リソースをすべて削除
	kubectl delete namespace micromart --ignore-not-found

k8s-status: ## Pod ステータスを確認
	kubectl get pods -n micromart

k8s-logs-%: ## 特定サービスのログを表示 (例: make k8s-logs-gateway)
	kubectl logs -n micromart -l app=$* -f --tail=100

# ── ユーティリティ ────────────────────────────────────────────────────────────────
setup: ## 初回セットアップ (依存ツールのインストール確認)
	@bash scripts/setup.sh

logs: ## Docker Compose のログを表示 (全サービス)
	$(COMPOSE_FULL) logs -f

logs-%: ## 特定サービスのログ (例: make logs-gateway)
	$(COMPOSE_FULL) logs -f $*

clean: ## ビルドキャッシュ・一時ファイルを削除
	@find . -name "*.pb.go" -path "*/gen/*" -delete
	@find . -name "coverage.out" -delete
	@find . -name "coverage.html" -delete
	docker system prune -f
