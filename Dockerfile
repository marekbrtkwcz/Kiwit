# Stage 1: Build Frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Go Backend
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
# sync-docs equivalent of `make sync-docs`: refresh the embedded /docs/ content
COPY docs/ /app/docs/
RUN mkdir -p ./internal/docs/content && cp /app/docs/*.md ./internal/docs/content/
COPY --from=frontend-builder /app/frontend/dist ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /kiwit ./cmd/server

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S kiwit && adduser -S kiwit -G kiwit
WORKDIR /app
RUN mkdir -p /app/data /app/uploads && chown -R kiwit:kiwit /app
COPY --from=backend-builder /kiwit /app/kiwit
USER kiwit
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz >/dev/null 2>&1 || exit 1
ENTRYPOINT ["/app/kiwit"]
