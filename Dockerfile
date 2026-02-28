# Build stage
FROM node:20-alpine AS web-builder
WORKDIR /app/web
COPY web/package*.json ./
RUN npm install
COPY web/ ./
RUN npm run build

# Go build stage
FROM golang:1.21-alpine AS go-builder
WORKDIR /app
RUN apk add --no-cache gcc g++ sqlite-dev
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /app/web/dist ./web/dist
RUN CGO_ENABLED=1 GOOS=linux go build -tags embed -ldflags="-w -s" -o model-router ./cmd/server

# Final stage
FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=go-builder /app/model-router .

# Create data directory
RUN mkdir -p /data

ENV PORT=8080
ENV HOST=0.0.0.0
ENV DB_PATH=/data/model-router.db

EXPOSE 8080

VOLUME ["/data"]

ENTRYPOINT ["./model-router"]
