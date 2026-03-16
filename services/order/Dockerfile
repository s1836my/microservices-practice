FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY proto/ ./proto/
COPY services/order/ ./services/order/

WORKDIR /build/services/order
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /order-service ./cmd/main.go

FROM gcr.io/distroless/static-debian12

COPY --from=builder /order-service /order-service

EXPOSE 50055 8084

ENTRYPOINT ["/order-service"]
