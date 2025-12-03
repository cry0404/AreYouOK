#!/bin/bash

set -e

# 获取当前脚本所在目录，确保无论在项目根目录还是其他目录执行都能找到 compose 文件
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/../docker/docker-compose.yml"

echo "Rebuilding database using compose file: ${COMPOSE_FILE} ..."

# 只关掉 postgres（避免影响其他服务，比如 api / worker / observability）
docker-compose -f "${COMPOSE_FILE}" down postgres

# 删除 Postgres 数据卷（会清空所有数据）
docker volume rm areyouok_postgres_data 2>/dev/null || true

# 仅启动 postgres，让它根据 docs/schema.sql 自动初始化
docker-compose -f "${COMPOSE_FILE}" up -d postgres

echo "Waiting for PostgreSQL to be ready..."
sleep 10

docker-compose -f "${COMPOSE_FILE}" logs postgres | tail -5
echo "Database rebuilt successfully!"