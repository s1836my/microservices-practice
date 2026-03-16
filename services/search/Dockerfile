FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY proto/ ./proto/
COPY services/search/ ./services/search/

WORKDIR /build/services/search
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /search-service ./cmd/main.go

FROM gcr.io/distroless/static-debian12

COPY --from=builder /search-service /search-service

EXPOSE 50053 8082

ENTRYPOINT ["/search-service"]
