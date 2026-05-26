#!/usr/bin/env bash
# 从 backup-meta-db.sh 产出的 custom dump 还原 meta-db（D11）。
# ⚠️ 破坏性：--clean --if-exists 会先删除现有对象再还原。请确认目标库可被覆盖。
#
# 用法：
#   bash scripts/restore-meta-db.sh backups/labeling_meta_20260526_023000.dump
#
# 环境变量同 backup（COMPOSE_FILE / SERVICE / DB / DB_USER）。
set -euo pipefail

DUMP="${1:-}"
if [ -z "$DUMP" ] || [ ! -f "$DUMP" ]; then
  echo "用法: bash scripts/restore-meta-db.sh <dump 文件>" >&2
  exit 2
fi

COMPOSE_FILE="${COMPOSE_FILE:-deployments/docker-compose.prod.yml}"
SERVICE="${SERVICE:-meta-db}"
DB="${DB:-labeling_meta}"
DB_USER="${DB_USER:-labeling}"

echo "[restore] ⚠️ 将用 ${DUMP} 覆盖 ${SERVICE}/${DB}（--clean --if-exists）"
read -r -p "确认继续？输入 yes： " ans
[ "$ans" = "yes" ] || { echo "已取消"; exit 1; }

docker compose -f "$COMPOSE_FILE" exec -T "$SERVICE" \
  pg_restore -U "$DB_USER" -d "$DB" --clean --if-exists --no-owner < "$DUMP"

echo "[restore] 完成。"
