FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY gen ./gen
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/warehouse ./cmd/warehouse

FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 warehouse
WORKDIR /app
COPY --from=build /out/warehouse /app/warehouse
USER warehouse
EXPOSE 50051
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 CMD nc -z 127.0.0.1 50051 || exit 1
ENTRYPOINT ["/app/warehouse"]
