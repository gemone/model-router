.PHONY: all build build-web dev clean run test test-web test-go test-integration test-e2e deps fmt lint docker-build

# 默认目标
all: build

# 构建 Web UI
build-web:
	cd web && npm install && npm run build
	cp -r web/dist internal/web/dist

# 构建 Go 服务端（嵌入模式）
build: build-web
	go build -tags embed -ldflags "-s -w" -o bin/model-router ./cmd/server

# 开发模式运行（不嵌入 UI）
dev:
	go run ./cmd/server

# 开发模式同时运行前端
dev-full:
	make -j2 dev-server dev-web

dev-server:
	go run ./cmd/server

dev-web:
	cd web && npm run dev

# 运行
run: build
	./bin/model-router

# 测试
test: test-go test-web

test-go:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-web:
	cd web && npm test

test-web-ui:
	cd web && npm run test:ui

test-web-coverage:
	cd web && npm run test:coverage

test-integration:
	go test -v -tags integration ./...

# E2E 测试
test-e2e:
	go test -v ./e2e/...

test-e2e-api:
	go test -v ./e2e/api/...

test-e2e-admin:
	go test -v ./e2e/admin/...

test-e2e-router:
	go test -v ./e2e/router/...

test-e2e-integration:
	go test -v ./e2e/integration/...

test-e2e-coverage:
	go test -v -coverprofile=e2e-coverage.out ./e2e/...
	go tool cover -html=e2e-coverage.out -o e2e-coverage.html

test-e2e-short:
	go test -v -short ./e2e/...

# E2E 测试（需要运行服务器）
test-e2e-ci:
	@echo "Starting server for E2E tests..."
	@trap 'kill $$(ps aux | grep "[g]o run ./cmd/server" | awk "{print \$$2}") 2>/dev/null || true' EXIT INT TERM; \
	go run ./cmd/server > /tmp/model-router-e2e.log 2>&1 & SERVER_PID=$$!; \
	echo "Server PID: $$SERVER_PID"; \
	sleep 3; \
	for i in $$(seq 1 30); do \
		if curl -f http://localhost:8080/health > /dev/null 2>&1; then \
			echo "Server is ready"; \
			break; \
		fi; \
		if [ $$i -eq 30 ]; then \
			echo "Server failed to start"; \
			cat /tmp/model-router-e2e.log; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	go test -v ./e2e/...; \
	TEST_RESULT=$$?; \
	exit $$TEST_RESULT

# 基准测试
bench:
	go test -bench=. -benchmem ./...

# 清理
clean:
	rm -rf bin/
	rm -rf web/dist/
	rm -rf web/node_modules/
	rm -f coverage.out coverage.html
	find . -type f -name "*.test" -delete

# 安装依赖
deps:
	go mod download
	go mod tidy
	cd web && npm install

# 更新依赖
update-deps:
	go get -u ./...
	go mod tidy
	cd web && npm update

# 代码格式化
fmt:
	go fmt ./...
	cd web && npm run lint 2>/dev/null || true

# 代码检查
lint:
	go vet ./...
	cd web && npm run lint

# 类型检查
typecheck:
	cd web && npm run typecheck

# Docker 构建
docker-build:
	docker build -t model-router:latest .

# Docker 运行
docker-run:
	docker run -p 8080:8080 -v $(PWD)/data:/data model-router:latest

# 发布版本
release:
	$(eval VERSION := $(shell git describe --tags --always --dirty))
	mkdir -p dist
	GOOS=darwin GOARCH=amd64 go build -tags embed -ldflags "-s -w -X main.version=$(VERSION)" -o dist/model-router-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 go build -tags embed -ldflags "-s -w -X main.version=$(VERSION)" -o dist/model-router-darwin-arm64 ./cmd/server
	GOOS=linux GOARCH=amd64 go build -tags embed -ldflags "-s -w -X main.version=$(VERSION)" -o dist/model-router-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=arm64 go build -tags embed -ldflags "-s -w -X main.version=$(VERSION)" -o dist/model-router-linux-arm64 ./cmd/server
	GOOS=windows GOARCH=amd64 go build -tags embed -ldflags "-s -w -X main.version=$(VERSION)" -o dist/model-router-windows-amd64.exe ./cmd/server

# 安全扫描
security-scan:
	go list -json -deps ./... | nancy sleuth 2>/dev/null || echo "nancy not installed"
	cd web && npm audit 2>/dev/null || true

# 生成证书
gen-certs:
	mkdir -p certs
	openssl req -x509 -newkey rsa:4096 -keyout certs/key.pem -out certs/cert.pem -days 365 -nodes -subj '/CN=localhost'

# 帮助
help:
	@echo "Available targets:"
	@echo "  make build              - Build the full application"
	@echo "  make dev                - Run in development mode"
	@echo "  make test               - Run all tests"
	@echo "  make test-go            - Run Go unit tests"
	@echo "  make test-web           - Run web tests"
	@echo "  make test-e2e           - Run E2E tests (requires server running)"
	@echo "  make test-e2e-api       - Run E2E API tests"
	@echo "  make test-e2e-admin     - Run E2E admin tests"
	@echo "  make test-e2e-router    - Run E2E router tests"
	@echo "  make test-e2e-ci        - Run E2E tests with auto server start"
	@echo "  make deps               - Install dependencies"
	@echo "  make update-deps        - Update all dependencies"
	@echo "  make clean              - Clean build artifacts"
	@echo "  make docker-build       - Build Docker image"
	@echo "  make release            - Build release binaries"
