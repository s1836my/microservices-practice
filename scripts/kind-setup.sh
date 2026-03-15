#!/usr/bin/env bash
# kind クラスタ + local registry セットアップ
# 参考: https://kind.sigs.k8s.io/docs/user/local-registry/

set -euo pipefail

CLUSTER_NAME="micromart"
REGISTRY_NAME="kind-registry"
REGISTRY_PORT="5001"

# K8s ノードイメージを安定版に固定
# 最新 kind が使う K8s 1.35 は Docker Desktop との相性問題あり → 1.31 を使用
# 利用可能なバージョン: https://github.com/kubernetes-sigs/kind/releases
NODE_IMAGE="kindest/node:v1.31.4"

# ── Colima / Docker リソース確認 ───────────────────────────────────────────
if command -v colima &>/dev/null; then
    COLIMA_MEM=$(colima list --json 2>/dev/null | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['memory']//1073741824)" 2>/dev/null || echo 0)
    if [[ "$COLIMA_MEM" -lt 6 ]]; then
        echo ""
        echo "[kind] ERROR: Colima memory is ${COLIMA_MEM}GiB (need 6GiB+)."
        echo "[kind] Run the following to restart with enough resources:"
        echo ""
        echo "    colima stop"
        echo "    colima start --cpu 4 --memory 8 --vm-type vz"
        echo ""
        exit 1
    fi
    echo "[kind] Colima memory: ${COLIMA_MEM}GiB OK"
fi

# ── Local Registry ─────────────────────────────────────────────────────────
if docker inspect "$REGISTRY_NAME" &>/dev/null && [ "$(docker inspect -f '{{.State.Running}}' "$REGISTRY_NAME")" = "true" ]; then
    echo "[kind] Local registry already running"
else
    # 停止済みコンテナが残っていれば削除
    docker rm -f "$REGISTRY_NAME" 2>/dev/null || true
    echo "[kind] Creating local registry..."
    docker run -d \
        --restart=always \
        -p "127.0.0.1:${REGISTRY_PORT}:5000" \
        --network bridge \
        --name "$REGISTRY_NAME" \
        registry:2
fi

# ── kind クラスタ ──────────────────────────────────────────────────────────
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo "[kind] Cluster '$CLUSTER_NAME' already exists"
else
    echo "[kind] Creating cluster '$CLUSTER_NAME' (K8s image: $NODE_IMAGE)..."
    cat <<EOF | kind create cluster --name "$CLUSTER_NAME" --image "$NODE_IMAGE" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
  - role: worker
containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${REGISTRY_PORT}"]
          endpoint = ["http://${REGISTRY_NAME}:5000"]
EOF
fi

# ── Registry を kind ネットワークに接続 ──────────────────────────────────
if ! docker network inspect kind 2>/dev/null | grep -q "$REGISTRY_NAME"; then
    docker network connect kind "$REGISTRY_NAME" 2>/dev/null || true
fi

# ── ConfigMap で registry を Kubernetes に登録 ────────────────────────────
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REGISTRY_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

# ── NGINX Ingress Controller ───────────────────────────────────────────────
echo "[kind] Installing NGINX Ingress Controller..."
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
    --for=condition=ready pod \
    --selector=app.kubernetes.io/component=controller \
    --timeout=120s

# ── metrics-server (HPA 用) ───────────────────────────────────────────────
echo "[kind] Installing metrics-server..."
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
# kind では TLS 検証をスキップ
kubectl patch deployment metrics-server -n kube-system \
    --type=json \
    -p='[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'

echo ""
echo "[kind] Setup complete!"
echo "  K8s version: $NODE_IMAGE"
echo "  Registry:    localhost:${REGISTRY_PORT}"
echo "  Cluster:     ${CLUSTER_NAME}"
echo ""
echo "  /etc/hosts に以下を追加してください:"
echo "  127.0.0.1 micromart.local"
