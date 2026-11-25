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
COPY --from=frontend-builder /app/frontend/dist ./internal/web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /kiwit ./cmd/server

# Stage 3: Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /kiwit /app/kiwit
EXPOSE 8080
ENTRYPOINT ["/app/kiwit"]
