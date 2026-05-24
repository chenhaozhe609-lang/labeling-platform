//go:build integration

// Package integration 是带真实 PostgreSQL 容器的端到端测试（store 层并发/幂等保证）。
// 运行：go test -tags=integration -race ./tests/integration/...
// 需要本机 Docker 可用（testcontainers 会起临时 postgres）。
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/platform/db"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

var (
	testPool  *pgxpool.Pool
	testStore *store.Store
	userSeq   atomic.Int64
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(90*time.Second)),
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
	testPool = pool
	testStore = store.New(pool)

	code := m.Run()

	pool.Close()
	_ = testcontainers.TerminateContainer(container)
	os.Exit(code)
}

// seed 创建 1 用户 + 1 数据集 + n 个 PENDING 任务，返回 userID / datasetID。
// 每个测试用独立 dataset，天然隔离（claim 按 dataset_id）。
func seed(t *testing.T, nTasks int) (userID, datasetID int64) {
	t.Helper()
	ctx := context.Background()
	userID = newUser(t)
	dsID, err := testStore.CreateDataset(ctx, &domain.Dataset{
		Name: "t", SourceSchema: "s", SourceTable: "t", SourcePKColumn: "id",
		FormSchema:        json.RawMessage(`{"version":1,"source_fields":[],"annotation_fields":[]}`),
		FormSchemaVersion: 1, Status: domain.StatusReady, TotalRows: nTasks,
	})
	if err != nil {
		t.Fatalf("create dataset: %v", err)
	}
	pks := make([]string, nTasks)
	for i := range pks {
		pks[i] = strconv.Itoa(i + 1)
	}
	if _, err := testStore.CreateTasks(ctx, dsID, pks); err != nil {
		t.Fatalf("create tasks: %v", err)
	}
	return userID, dsID
}

func newUser(t *testing.T) int64 {
	t.Helper()
	name := fmt.Sprintf("u%d", userSeq.Add(1))
	u, err := testStore.CreateUser(context.Background(), name, "x", domain.RoleAnnotator)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u.ID
}
