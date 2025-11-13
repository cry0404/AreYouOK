.PHONY: gen help format fmt

help: 
	@echo "Available commands:"
	@echo "  make gen      - 生成 GORM Gen 代码"
	@echo "  make format   - 格式化所有代码（go fmt + goimports）"
	@echo "  make fmt      - 快速格式化（仅 go fmt）"
	@echo "  make lint     - 运行代码检查"
	@echo "  make lint-fix - 运行代码检查并自动修复"

gen: 
	@echo "Generating GORM Gen code..."
	@go run cmd/gen/main.go

run:
	@echo "Server run"
	@go run cmd/server/server.go

# 格式化代码（go fmt + goimports）
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

# 快速格式化（仅 go fmt）
fmt:
	@echo "Formatting code with go fmt..."
	@go fmt ./...
	@echo "Formatting complete!"

lint:
	@golangci-lint run

lint-fix:
	@golangci-lint run --fix