#!/usr/bin/env bash
# meta-db 每日备份（D11）：经 docker compose 对 meta-db 跑 pg_dump（custom 格式），
# 落到 backups/ 并按天轮转。source-db 是沙箱（用户上传的可重建数据），不在备份范围。
#
# 用法：
#   bash scripts/backup-meta-db.sh
#
# 可调环境变量（含默认值）：
#   COMPOSE_FILE=deployments/docker-compose.prod.yml   # 目标编排（dev 用 deployments/docker-compose.yml）
#   SERVICE=meta-db          DB=labeling_meta           DB_USER=labeling
#   OUT_DIR=backups          KEEP_DAYS=7                # 保留天数，更早的删除
#
# 定时（每天 02:30）：
#   crontab -e  →  30 2 * * *  cd /path/to/labeling-platform && bash scripts/backup-meta-db.sh >> backups/backup.log 2>&1
#   Windows 任务计划程序：操作 = `bash` 参数 = `scripts/backup-meta-db.sh`，起始于仓库根目录。
#
# 还原见 scripts/restore-meta-db.sh。
set -euo pipefail

COMPOSE_FILE="${COMPOSE_FILE:-deployments/docker-compose.prod.yml}"
SERVICE="${SERVICE:-meta-db}"
DB="${DB:-labeling_meta}"
DB_USER="${DB_USER:-labeling}"
OUT_DIR="${OUT_DIR:-backups}"
KEEP_DAYS="${KEEP_DAYS:-7}"

ts="$(date +%Y%m%d_%H%M%S)"
out="${OUT_DIR}/${DB}_${ts}.dump"
mkdir -p "$OUT_DIR"

echo "[backup] pg_dump ${SERVICE}/${DB} → ${out}"
# -Fc custom 格式：压缩 + 支持 pg_restore 选择性还原。-T 不分配 TTY，便于管道写文件。
docker compose -f "$COMPOSE_FILE" exec -T "$SERVICE" \
  pg_dump -U "$DB_USER" -Fc "$DB" > "$out"

size="$(wc -c < "$out" | tr -d ' ')"
if [ "$size" -lt 100 ]; then
  echo "[backup] 失败：产出过小（${size}B），可能 meta-db 未就绪或凭据错误" >&2
  rm -f "$out"
  exit 1
fi
echo "[backup] 完成：${out}（${size} 字节）"

# 轮转：删除超过 KEEP_DAYS 天的备份
deleted="$(find "$OUT_DIR" -maxdepth 1 -name "${DB}_*.dump" -mtime "+${KEEP_DAYS}" -print -delete | wc -l | tr -d ' ')"
echo "[backup] 轮转：删除 ${deleted} 个超过 ${KEEP_DAYS} 天的旧备份；现存 $(find "$OUT_DIR" -maxdepth 1 -name "${DB}_*.dump" | wc -l | tr -d ' ') 份"
