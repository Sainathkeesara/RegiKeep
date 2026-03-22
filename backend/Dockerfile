# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src

# Copy all source first, then tidy generates go.sum inside the container.
# (Splitting go.mod/go.sum for cache doesn't help without a committed go.sum.)
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /out/rgk ./cmd/rgk

# ── Final stage ───────────────────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

# Non-root user
RUN addgroup -S rgk && adduser -S rgk -G rgk

WORKDIR /app
COPY --from=builder /out/rgk /usr/local/bin/rgk

# Data directory for SQLite
RUN mkdir -p /data && chown rgk:rgk /data

USER rgk

EXPOSE 8080

ENTRYPOINT ["rgk"]
CMD ["serve"]
