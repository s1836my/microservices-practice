FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy workspace files
COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY proto/ ./proto/
COPY services/product/ ./services/product/

# Build
WORKDIR /build/services/product
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /product-service ./cmd/main.go

# Runtime
FROM gcr.io/distroless/static-debian12

COPY --from=builder /product-service /product-service

EXPOSE 50052 8081

ENTRYPOINT ["/product-service"]
