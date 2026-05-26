//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// orgFixture 是一个组织的完整切片：owner(admin) + annotator + reviewer + 1 数据集
// （pk"1" 已完成并有有效标注，pk"2" 仍 PENDING）。用于验证跨组织隔离。
type orgFixture struct {
	org       int64
	admin     int64
	annotator int64
	reviewer  int64
	dataset   int64
	task      int64 // 已完成的任务（pk"1"）
	ann       int64 // 该任务上的有效标注
}

func orgUser(t *testing.T, orgID int64, role domain.Role) int64 {
	t.Helper()
	u, err := testStore.CreateUser(context.Background(), store.NewUser{
		Username: "u", Email: uniqueEmail("u"), PasswordHash: "x", Role: role, OrgID: &orgID,
	})
	if err != nil {
		t.Fatalf("create org user: %v", err)
	}
	return u.ID
}

func makeOrgFixture(t *testing.T, name string) *orgFixture {
	t.Helper()
	ctx := context.Background()
	org, owner, err := testStore.CreateOrgWithOwner(ctx, name,
		store.NewUser{Username: "admin", Email: uniqueEmail("admin"), PasswordHash: "x"})
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	f := &orgFixture{
		org:       org.ID,
		admin:     owner.ID,
		annotator: orgUser(t, org.ID, domain.RoleAnnotator),
		reviewer:  orgUser(t, org.ID, domain.RoleReviewer),
		dataset:   newDatasetInOrg(t, &org.ID, name+"-ds"),
	}
	if _, err := testStore.CreateTasks(ctx, f.dataset, []string{"1", "2"}); err != nil {
		t.Fatalf("create tasks: %v", err)
	}
	// annotator 领取并提交 pk"1" → COMPLETED + 有效标注；pk"2" 留作 PENDING（供 claim 测试）。
	tk, err := testStore.ClaimTask(ctx, f.dataset, f.annotator, 30, &org.ID)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	f.task = tk.ID
	if err := testStore.SubmitAnnotation(ctx, tk.ID, f.annotator, json.RawMessage(`{"fills":{"x":"v"}}`), 1); err != nil {
		t.Fatalf("submit: %v", err)
	}
	f.ann = annIDForTask(t, tk.ID)
	return f
}

// claim 面：不能领取他组织数据集的任务；本组织/超管可领。
func TestOrgIsolation_Claim(t *testing.T) {
	ctx := context.Background()
	a := makeOrgFixture(t, "claimA")
	b := makeOrgFixture(t, "claimB")

	if _, err := testStore.ClaimTask(ctx, b.dataset, a.annotator, 30, &a.org); !errors.Is(err, store.ErrNoTask) {
		t.Fatalf("A 跨组织领 B 的任务应 ErrNoTask，got %v", err)
	}
	tk, err := testStore.ClaimTask(ctx, b.dataset, b.annotator, 30, &b.org)
	if err != nil || tk == nil {
		t.Fatalf("B 领本组织任务应成功，got tk=%v err=%v", tk, err)
	}
}

// get task 面：跨组织按 id 取任务视同不存在；本组织/超管可取。
func TestOrgIsolation_GetTask(t *testing.T) {
	ctx := context.Background()
	a := makeOrgFixture(t, "gettaskA")
	b := makeOrgFixture(t, "gettaskB")

	if _, err := testStore.GetTask(ctx, b.task, &a.org); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("A 跨组织取 B 任务应 ErrNotFound，got %v", err)
	}
	if _, err := testStore.GetTask(ctx, b.task, &b.org); err != nil {
		t.Fatalf("B 取本组织任务应成功，got %v", err)
	}
	if _, err := testStore.GetTask(ctx, b.task, nil); err != nil {
		t.Fatalf("超管取任意任务应成功，got %v", err)
	}
}

// 数据集面（export/detail/sync/generate/pause/resume/form-schema 都经 GetDataset 护栏）。
func TestOrgIsolation_DatasetGate(t *testing.T) {
	ctx := context.Background()
	a := makeOrgFixture(t, "dsgateA")
	b := makeOrgFixture(t, "dsgateB")

	if _, err := testStore.GetDataset(ctx, b.dataset, &a.org); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("A 跨组织取 B 数据集应 ErrNotFound，got %v", err)
	}
	if _, err := testStore.GetDataset(ctx, b.dataset, &b.org); err != nil {
		t.Fatalf("B 取本组织数据集应成功，got %v", err)
	}
	if _, err := testStore.GetDataset(ctx, b.dataset, nil); err != nil {
		t.Fatalf("超管取任意数据集应成功，got %v", err)
	}
}

// review 面：reviewer 不能裁决/改写他组织的标注；本组织 reviewer 可裁决。
func TestOrgIsolation_Review(t *testing.T) {
	ctx := context.Background()
	a := makeOrgFixture(t, "reviewA")
	b := makeOrgFixture(t, "reviewB")

	if err := testStore.SubmitReview(ctx, b.ann, a.reviewer, "approved", "", &a.org); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("A 跨组织裁决 B 标注应 ErrConflict，got %v", err)
	}
	if err := testStore.EditReview(ctx, b.ann, a.reviewer, json.RawMessage(`{"fills":{"x":"z"}}`), "", &a.org); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("A 跨组织改写 B 标注应 ErrConflict，got %v", err)
	}
	// 本组织 reviewer（非作者）可裁决。
	if err := testStore.SubmitReview(ctx, b.ann, b.reviewer, "approved", "", &b.org); err != nil {
		t.Fatalf("B reviewer 裁决本组织标注应成功，got %v", err)
	}
}

// dashboard 面：各组织看板只统计本组织；超管跨组织。
func TestOrgIsolation_Dashboard(t *testing.T) {
	ctx := context.Background()
	a := makeOrgFixture(t, "dashA")
	b := makeOrgFixture(t, "dashB")

	da, err := testStore.GetDashboard(ctx, &a.org)
	if err != nil {
		t.Fatalf("dashboard A: %v", err)
	}
	if da.Datasets != 1 || da.Completed != 1 || da.Pending != 1 {
		t.Fatalf("组织 A 看板应只见自身：datasets=%d completed=%d pending=%d（期望 1/1/1）",
			da.Datasets, da.Completed, da.Pending)
	}

	db, err := testStore.GetDashboard(ctx, &b.org)
	if err != nil {
		t.Fatalf("dashboard B: %v", err)
	}
	if db.Datasets != 1 {
		t.Fatalf("组织 B 看板 datasets=%d，期望 1", db.Datasets)
	}

	all, err := testStore.GetDashboard(ctx, nil) // 超管跨组织
	if err != nil {
		t.Fatalf("dashboard all: %v", err)
	}
	if all.Datasets < 2 || all.Completed < 2 {
		t.Fatalf("超管看板应跨组织聚合：datasets=%d completed=%d（期望 ≥2）", all.Datasets, all.Completed)
	}
}

// users 面：组织用户列表/末位 admin 计数按 org 隔离；超管跨组织。
func TestOrgIsolation_Users(t *testing.T) {
	ctx := context.Background()
	a := makeOrgFixture(t, "usersA")
	makeOrgFixture(t, "usersB")

	usersA, err := testStore.ListUsers(ctx, &a.org)
	if err != nil {
		t.Fatalf("list users A: %v", err)
	}
	if len(usersA) != 3 { // owner(admin) + annotator + reviewer
		t.Fatalf("组织 A 用户数=%d，期望 3", len(usersA))
	}
	for _, u := range usersA {
		if u.OrgID == nil || *u.OrgID != a.org {
			t.Fatalf("ListUsers(A) 含非本组织用户 #%d (org=%v)", u.ID, u.OrgID)
		}
	}
	if n, _ := testStore.CountAdmins(ctx, &a.org); n != 1 {
		t.Fatalf("组织 A admin 数=%d，期望 1", n)
	}

	all, err := testStore.ListUsers(ctx, nil) // 超管
	if err != nil {
		t.Fatalf("list users all: %v", err)
	}
	if len(all) < 6 {
		t.Fatalf("超管用户列表应跨组织：len=%d（期望 ≥6）", len(all))
	}
}
