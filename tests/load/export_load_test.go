//go:build load

// Package load 是手动触发的压测（AC-9，PRD §19）。默认不在常规 -race 套件里跑。
//
// 运行（默认 10 万行，约数秒）：
//
//	go test -tags=load -run TestExport_Streaming ./tests/load/...
//
// 跑真·百万行（PRD AC-9）：
//
//	LOAD_ROWS=1000000 go test -tags=load -run TestExport_Streaming -timeout 600s -v ./tests/load/...
//
// 验证点：导出走 store.StreamExport（meta-db 游标流式）+ 分块 source.GetRows，
// 整页 chunk=500 滚动 flush 到 io.Discard——内存不随行数线性增长（本测试断言堆增量有界）。
package load

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/platform/db"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/source"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

const exportChunk = 500 // 与 handler.exportChunk 对齐

var (
	testPool *pgxpool.Pool
	testSt   *store.Store
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("test"), postgres.WithUsername("test"), postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(120*time.Second)),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "启动 postgres 容器失败:", err)
		os.Exit(1)
	}
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "取连接串失败:", err)
		os.Exit(1)
	}
	if err := db.Migrate(connStr, "file://../../migrations", false); err != nil {
		fmt.Fprintln(os.Stderr, "迁移失败:", err)
		os.Exit(1)
	}
	pool, err := db.NewPool(ctx, connStr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "连接失败:", err)
		os.Exit(1)
	}
	testPool, testSt = pool, store.New(pool)

	code := m.Run()
	pool.Close()
	_ = testcontainers.TerminateContainer(container)
	os.Exit(code)
}

func TestExport_Streaming(t *testing.T) {
	ctx := context.Background()
	n := envInt("LOAD_ROWS", 100_000)
	t.Logf("行数 N=%d", n)

	// 用户 + 数据集（源坐标指向下面 bulk 造的源表）
	org := int64(1)
	u, err := testSt.CreateUser(ctx, store.NewUser{
		Username: "loadu", Email: "loadu@t.local", PasswordHash: "x", Role: domain.RoleAnnotator, OrgID: &org,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	const srcSchema, srcTable = "srcload", "t"
	dsID, err := testSt.CreateDataset(ctx, &domain.Dataset{
		Name: "load", SourceSchema: srcSchema, SourceTable: srcTable, SourcePKColumn: "id",
		FormSchema:        json.RawMessage(`{"version":1,"primary_cols":["title"],"columns":[{"code":"id","type":"integer","role":"id"},{"code":"title","type":"text","role":"context"},{"code":"label","type":"text","role":"fill"}]}`),
		FormSchemaVersion: 1, Status: domain.StatusReady, TotalRows: n,
	})
	if err != nil {
		t.Fatalf("create dataset: %v", err)
	}

	// bulk 造数据：源表 N 行 + N 个 COMPLETED 任务 + N 条有效 annotation（generate_series，秒级）
	mustExec(t, `CREATE SCHEMA `+srcSchema)
	mustExec(t, `CREATE TABLE `+srcSchema+`.`+srcTable+` (id int PRIMARY KEY, title text)`)
	mustExec(t, `INSERT INTO `+srcSchema+`.`+srcTable+` SELECT g, 'title '||g FROM generate_series(1,$1) g`, n)
	mustExec(t, `INSERT INTO tasks (dataset_id, source_row_pk, content_hash, status, completed_at)
	             SELECT $1, g::text, 'h'||g, 'COMPLETED', now() FROM generate_series(1,$2) g`, dsID, n)
	mustExec(t, `INSERT INTO annotations (task_id, dataset_id, user_id, data, form_schema_version, round)
	             SELECT t.id, $1, $2, '{"fills":{"label":"v"},"_source":"human"}'::jsonb, 1, 1
	             FROM tasks t WHERE t.dataset_id=$1`, dsID, u.ID)

	rdr := source.New(testPool)
	enc := json.NewEncoder(io.Discard)

	var beforeHeap runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&beforeHeap)
	start := time.Now()

	// 复刻 handler 的流式导出：StreamExport 游标 + 整页 chunk 批量取源行 + 写出丢弃。
	var count int
	buf := make([]*store.ExportRow, 0, exportChunk)
	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		pks := make([]string, len(buf))
		for i, r := range buf {
			pks[i] = r.PK
		}
		srcRows, err := rdr.GetRows(ctx, srcSchema, srcTable, "id", pks)
		if err != nil {
			return err
		}
		for _, r := range buf {
			rec := srcRows[r.PK]
			if rec == nil {
				rec = map[string]any{}
			}
			var fills map[string]any
			_ = json.Unmarshal(r.Fills, &fills)
			rec["label"] = fills["label"] // fill 列叠加
			rec["_pk"] = r.PK
			if err := enc.Encode(rec); err != nil {
				return err
			}
			count++
		}
		buf = buf[:0]
		return nil
	}
	err = testSt.StreamExport(ctx, dsID, false, func(r *store.ExportRow) error {
		buf = append(buf, r)
		if len(buf) >= exportChunk {
			return flush()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("StreamExport: %v", err)
	}
	if err := flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	elapsed := time.Since(start)

	var afterHeap runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&afterHeap)

	if count != n {
		t.Fatalf("导出行数=%d，期望 %d", count, n)
	}

	heapDelta := int64(afterHeap.HeapInuse) - int64(beforeHeap.HeapInuse)
	rate := float64(n) / elapsed.Seconds()
	t.Logf("导出 %d 行用时 %s（%.0f 行/秒），堆增量 %.1f MB（chunk=%d 滚动）",
		n, elapsed.Round(time.Millisecond), rate, float64(heapDelta)/(1<<20), exportChunk)

	// 内存有界：流式导出的活跃堆不应随 N 线性增长。给一个宽松上限——
	// 若退化成「全部行加载进内存」，百万行会到数百 MB+，此断言即可发现回归。
	const heapCap = 96 << 20 // 96MB
	if heapDelta > heapCap {
		t.Errorf("导出后堆增量 %.1f MB 超过上限 %d MB——疑似未真正流式（累积了全部行）",
			float64(heapDelta)/(1<<20), heapCap>>20)
	}
}

func mustExec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := testPool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}
