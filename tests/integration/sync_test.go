//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/service"
)

// ---- 增量同步验收（AC-5..8，PRD §12 / §19）----
// 直接打 store.SyncTasks（生产用的单事务 CTE）+ service.FetchHashedRows（content_hash 计算），
// 不经 pgrestore/沙箱——把「reconcile (pk,hash)」这条核心路径测准、测快。

func newSyncDataset(t *testing.T, hashCols []string) int64 {
	t.Helper()
	id, err := testStore.CreateDataset(context.Background(), &domain.Dataset{
		Name: "sync", SourceSchema: "s", SourceTable: "t", SourcePKColumn: "id",
		HashColumns:       hashCols,
		FormSchema:        json.RawMessage(`{"version":1,"primary_cols":[],"columns":[]}`),
		FormSchemaVersion: 1, Status: domain.StatusReady,
	})
	if err != nil {
		t.Fatalf("create dataset: %v", err)
	}
	return id
}

func newBatch(t *testing.T, dsID int64) int64 {
	t.Helper()
	b, err := testStore.CreateImportBatch(context.Background(), dsID, "dump.sql", 0, nil)
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	return b
}

// taskByPK 取某 dataset 下指定 pk 的任务状态。
func taskByPK(t *testing.T, dsID int64, pk string) (id int64, status string, round int, completedNull bool) {
	t.Helper()
	if err := testPool.QueryRow(context.Background(),
		`SELECT id, status, round, completed_at IS NULL FROM tasks WHERE dataset_id=$1 AND source_row_pk=$2`,
		dsID, pk).Scan(&id, &status, &round, &completedNull); err != nil {
		t.Fatalf("task by pk %s: %v", pk, err)
	}
	return
}

func taskCount(t *testing.T, dsID int64) int {
	t.Helper()
	var n int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*) FROM tasks WHERE dataset_id=$1`, dsID).Scan(&n); err != nil {
		t.Fatalf("count tasks: %v", err)
	}
	return n
}

// allSuperseded 该 task 的 annotation 是否全部已标 superseded（每 task 这里恰 1 条）。
func allSuperseded(t *testing.T, taskID int64) bool {
	t.Helper()
	var total, sup int
	if err := testPool.QueryRow(context.Background(),
		`SELECT count(*), count(*) FILTER (WHERE superseded_at IS NOT NULL) FROM annotations WHERE task_id=$1`,
		taskID).Scan(&total, &sup); err != nil {
		t.Fatalf("count annotations: %v", err)
	}
	return total > 0 && sup == total
}

// completeWithAnnotation 直接把任务置 COMPLETED + 插一条有效 annotation（模拟既有标注）。
func completeWithAnnotation(t *testing.T, taskID, dsID, uid int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := testPool.Exec(ctx,
		`UPDATE tasks SET status='COMPLETED', completed_at=now() WHERE id=$1`, taskID); err != nil {
		t.Fatalf("complete task: %v", err)
	}
	if _, err := testPool.Exec(ctx,
		`INSERT INTO annotations (task_id, dataset_id, user_id, data, form_schema_version, round)
		 VALUES ($1,$2,$3,'{}',1,1)`, taskID, dsID, uid); err != nil {
		t.Fatalf("insert annotation: %v", err)
	}
}

// AC-5：重复上传同一备份 → 任务总数不变，无新增、无重标。
func TestSync_AC5_IdempotentReupload(t *testing.T) {
	ctx := context.Background()
	dsID := newSyncDataset(t, nil)
	pks := []string{"1", "2", "3"}
	hs := []string{"hA", "hB", "hC"}
	if n, err := testStore.InsertTasksWithHash(ctx, dsID, newBatch(t, dsID), pks, hs); err != nil || n != 3 {
		t.Fatalf("首次入库 n=%d err=%v，期望 3", n, err)
	}

	newN, updN, err := testStore.SyncTasks(ctx, dsID, newBatch(t, dsID), pks, hs)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if newN != 0 || updN != 0 {
		t.Errorf("重传同备份 new=%d updated=%d，期望 0/0", newN, updN)
	}
	if c := taskCount(t, dsID); c != 3 {
		t.Errorf("任务数=%d，期望 3（幂等）", c)
	}
}

// AC-6：含 K 条新行的备份 → 恰好新增 K 个任务（差异 = K）。
func TestSync_AC6_IncrementalNewRows(t *testing.T) {
	ctx := context.Background()
	dsID := newSyncDataset(t, nil)
	testStore.InsertTasksWithHash(ctx, dsID, newBatch(t, dsID), []string{"1", "2", "3"}, []string{"hA", "hB", "hC"})

	// 原 3 行不变 + 新增 2 行（K=2）
	newN, updN, err := testStore.SyncTasks(ctx, dsID, newBatch(t, dsID),
		[]string{"1", "2", "3", "4", "5"}, []string{"hA", "hB", "hC", "hD", "hE"})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if newN != 2 || updN != 0 {
		t.Errorf("增量 new=%d updated=%d，期望 2/0", newN, updN)
	}
	if c := taskCount(t, dsID); c != 5 {
		t.Errorf("任务数=%d，期望 5", c)
	}
	if _, st, _, _ := taskByPK(t, dsID, "4"); st != "PENDING" {
		t.Errorf("新行 pk=4 status=%s，期望 PENDING", st)
	}
}

// AC-7：已标注行内容被修改 → 该行回 PENDING + round+1 + 旧 annotation superseded；未变行不动。
func TestSync_AC7_ContentChangeRelabels(t *testing.T) {
	ctx := context.Background()
	uid := newUser(t)
	dsID := newSyncDataset(t, nil)
	pks := []string{"1", "2", "3"}
	testStore.InsertTasksWithHash(ctx, dsID, newBatch(t, dsID), pks, []string{"hA", "hB", "hC"})
	for _, pk := range pks {
		id, _, _, _ := taskByPK(t, dsID, pk)
		completeWithAnnotation(t, id, dsID, uid)
	}

	// 新 dump：仅 pk=2 内容改变（hash 不同）
	newN, updN, err := testStore.SyncTasks(ctx, dsID, newBatch(t, dsID), pks, []string{"hA", "hB_CHANGED", "hC"})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if newN != 0 || updN != 1 {
		t.Errorf("改 1 行 new=%d updated=%d，期望 0/1", newN, updN)
	}

	id2, st2, r2, cn2 := taskByPK(t, dsID, "2")
	if st2 != "PENDING" || r2 != 2 || !cn2 {
		t.Errorf("改动行 pk=2 status=%s round=%d completedNull=%v，期望 PENDING/2/true", st2, r2, cn2)
	}
	if !allSuperseded(t, id2) {
		t.Errorf("改动行 pk=2 的 annotation 未标 superseded")
	}

	for _, pk := range []string{"1", "3"} {
		id, st, r, _ := taskByPK(t, dsID, pk)
		if st != "COMPLETED" || r != 1 {
			t.Errorf("未变行 pk=%s status=%s round=%d，期望 COMPLETED/1", pk, st, r)
		}
		if allSuperseded(t, id) {
			t.Errorf("未变行 pk=%s 的 annotation 不应 superseded", pk)
		}
	}
}

// AC-8：新备份增加字段（不在 hash_columns）→ content_hash 不变 → 不触发任何重标。
func TestSync_AC8_SchemaEvolutionNoRelabel(t *testing.T) {
	ctx := context.Background()
	const schema = "src_ac8"
	if _, err := testPool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS `+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() { testPool.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`) })
	if _, err := testPool.Exec(ctx, `CREATE TABLE `+schema+`.t (id int PRIMARY KEY, title text, body text)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := testPool.Exec(ctx, `INSERT INTO `+schema+`.t VALUES (1,'a','x'),(2,'b','y')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	hashCols := []string{"title", "body"}
	before, err := service.FetchHashedRows(ctx, testPool, schema, "t", "id", hashCols)
	if err != nil {
		t.Fatalf("fetch before: %v", err)
	}

	// schema 演进：加一个不在 hash_columns 的新列（旧行该列为 NULL）
	if _, err := testPool.Exec(ctx, `ALTER TABLE `+schema+`.t ADD COLUMN extra text`); err != nil {
		t.Fatalf("alter add column: %v", err)
	}
	after, err := service.FetchHashedRows(ctx, testPool, schema, "t", "id", hashCols)
	if err != nil {
		t.Fatalf("fetch after: %v", err)
	}

	// 同 pk 的 content_hash 必须不变（加列不进 hash_columns）
	bm := map[string]string{}
	for _, r := range before {
		bm[r.PK] = r.Hash
	}
	for _, r := range after {
		if bm[r.PK] != r.Hash {
			t.Errorf("加列后 pk=%s content_hash 变了：%s → %s", r.PK, bm[r.PK], r.Hash)
		}
	}

	// 端到端：before 入库后，用 after 同步 → 不应有任何新增/重标
	dsID := newSyncDataset(t, hashCols)
	bpks, bhs := splitRows(before)
	testStore.InsertTasksWithHash(ctx, dsID, newBatch(t, dsID), bpks, bhs)
	apks, ahs := splitRows(after)
	newN, updN, err := testStore.SyncTasks(ctx, dsID, newBatch(t, dsID), apks, ahs)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if newN != 0 || updN != 0 {
		t.Errorf("加列后 sync new=%d updated=%d，期望 0/0（schema 演进不重标）", newN, updN)
	}
}

func splitRows(rows []service.HashedRow) (pks, hashes []string) {
	pks = make([]string, len(rows))
	hashes = make([]string, len(rows))
	for i, r := range rows {
		pks[i], hashes[i] = r.PK, r.Hash
	}
	return
}
