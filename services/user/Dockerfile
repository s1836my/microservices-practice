FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy workspace files
COPY go.work go.work.sum ./
COPY pkg/ ./pkg/
COPY proto/ ./proto/
COPY services/user/ ./services/user/

# Build
WORKDIR /build/services/user
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /user-service ./cmd/main.go

# Runtime
FROM gcr.io/distroless/static-debian12

COPY --from=builder /user-service /user-service

EXPOSE 50051 8080

ENTRYPOINT ["/user-service"]
