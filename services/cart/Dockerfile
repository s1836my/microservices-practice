FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY proto/ ./proto/
COPY services/cart/ ./services/cart/

WORKDIR /build/services/cart
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /cart-service ./cmd/main.go

FROM gcr.io/distroless/static-debian12

COPY --from=builder /cart-service /cart-service

EXPOSE 50054 8083

ENTRYPOINT ["/cart-service"]
