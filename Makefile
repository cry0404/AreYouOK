.PHONY: gen help format fmt run-api run-worker build-api build-worker test test-producer test-consumer test-v test-cover

help:
	@echo "Available commands:"
	@echo "  make gen         - 生成 GORM Gen 代码"
	@echo "  make run-api     - 启动 API 服务"
	@echo "  make run-worker  - 启动 Worker 服务"
	@echo "  make build-api   - 构建 API 服务二进制文件"
	@echo "  make build-worker - 构建 Worker 服务二进制文件"
	@echo "  make format      - 格式化所有代码（go fmt + goimports）"
	@echo "  make fmt         - 快速格式化（仅 go fmt）"
	@echo "  make lint        - 运行代码检查"
	@echo "  make lint-fix    - 运行代码检查并自动修复"

gen: 
	@echo "Generating GORM Gen code..."
	@go run cmd/gen/main.go

run-api:
	@echo "Starting API service..."
	@go run cmd/server/server.go


run-worker:
	@echo "Starting Worker service..."
	@go run cmd/worker/worker.go


build-api:
	@echo "Building API service..."
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api ./cmd/server
	@echo "API service built: bin/api"


build-worker:
	@echo "Building Worker service..."
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/worker ./cmd/worker
	@echo "Worker service built: bin/worker"


run:
	@echo "Server run (deprecated, use 'make run-api' instead)"
	@go run cmd/server/server.go


format:
	@echo "Formatting code with go fmt..."
	@go fmt ./...
	@echo "Formatting imports with goimports..."
	@if command -v goimports > /dev/null; then \
		goimports -w .; \
	else \
		echo "goimports not found, installing..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
		goimports -w .; \
	fi
	@echo "Formatting complete!"


fmt:
	@echo "Formatting code with go fmt..."
	@go fmt ./...
	@echo "Formatting complete!"

lint:
	@golangci-lint run

lint-fix:
	@golangci-lint run --fix

