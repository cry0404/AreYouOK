#!/bin/bash

set -euo pipefail

# 进入项目根目录（脚本位于 scripts/ 下）
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${SCRIPT_DIR}/.."
cd "${PROJECT_ROOT}"

echo "[run-checkin-scheduler-once] Running single daily check-in scheduling via go test ..."

# 使用已有的测试用例触发一次调度逻辑：
# - 初始化 logger / storage / snowflake
# - 调用 schedule.GetScheduler().ScheduleDailyCheckIns(...)
go test ./test -run TestScheduleDailyCheckInsOnce -v

echo "[run-checkin-scheduler-once] Done."


