.PHONY: gen help

help: 
	@echo "Available commands:"
	@echo "  make gen      - 生成 GORM Gen 代码"

gen: 
	@echo "Generating GORM Gen code..."
	@go run cmd/gen/main.go

run:
	@echo "Server run"
	@go run cmd/server/server.go