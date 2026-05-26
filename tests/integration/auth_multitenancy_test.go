//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/platform/jwt"
	"github.com/chenhaozhe609-lang/labeling-platform/internal/repository/store"
)

// uniqueEmail 在共享测试库里生成不冲突的邮箱。
func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s%d@org.test", prefix, userSeq.Add(1))
}

// newOrg 注册一个组织 + owner（admin），返回 owner 用户。
func newOrg(t *testing.T, name string) *domain.User {
	t.Helper()
	_, owner, err := testStore.CreateOrgWithOwner(context.Background(), name,
		store.NewUser{Username: "owner", Email: uniqueEmail("owner"), PasswordHash: "x"})
	if err != nil {
		t.Fatalf("create org %q: %v", name, err)
	}
	if owner.OrgID == nil {
		t.Fatal("owner 应归属新建组织")
	}
	if owner.Role != domain.RoleAdmin {
		t.Fatalf("owner 角色应为 admin，got %s", owner.Role)
	}
	return owner
}

// ① 邮箱全局唯一：同邮箱二次注册组织 → ErrConflict，事务回滚不留脏数据。
func TestSignup_DuplicateEmailConflict(t *testing.T) {
	ctx := context.Background()
	email := uniqueEmail("dup")

	if _, _, err := testStore.CreateOrgWithOwner(ctx, "org-dup-1",
		store.NewUser{Username: "a", Email: email, PasswordHash: "x"}); err != nil {
		t.Fatalf("首次注册应成功: %v", err)
	}
	_, _, err := testStore.CreateOrgWithOwner(ctx, "org-dup-2",
		store.NewUser{Username: "b", Email: email, PasswordHash: "x"})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("同邮箱二次注册应 ErrConflict，got %v", err)
	}
	// 大小写不敏感唯一。
	_, _, err = testStore.CreateOrgWithOwner(ctx, "org-dup-3",
		store.NewUser{Username: "c", Email: upper(email), PasswordHash: "x"})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("大小写变体也应冲突，got %v", err)
	}
}

// ② token_version 吊销：bump 后旧 refresh 携带的 tv 与库内不再相等（Refresh 据此拒签）。
func TestTokenVersion_RevokesOldRefresh(t *testing.T) {
	ctx := context.Background()
	owner := newOrg(t, "org-tv")
	jm := jwt.NewManager("test-secret", 15*time.Minute, 24*time.Hour)

	_, refresh, err := jm.Generate(owner.ID, string(owner.Role), owner.OrgID, owner.TokenVersion)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	claims, err := jm.Parse(refresh)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// 旧 token 此刻仍有效：tv 相等。
	cur, err := testStore.GetUserByID(ctx, owner.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if claims.TokenVersion != cur.TokenVersion {
		t.Fatalf("吊销前 tv 应相等：token=%d db=%d", claims.TokenVersion, cur.TokenVersion)
	}

	// logout-all / 改密 → bump。
	if err := testStore.BumpTokenVersion(ctx, owner.ID); err != nil {
		t.Fatalf("bump: %v", err)
	}
	reloaded, err := testStore.GetUserByID(ctx, owner.ID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if reloaded.TokenVersion != cur.TokenVersion+1 {
		t.Fatalf("bump 应 +1：%d → %d", cur.TokenVersion, reloaded.TokenVersion)
	}
	if claims.TokenVersion == reloaded.TokenVersion {
		t.Fatal("旧 refresh 的 tv 应与库内不再相等（被吊销）")
	}
}

// ③ org 隔离：各组织只看到本组织数据集；超管（nil）跨组织可见两者。
func TestListDatasets_OrgIsolation(t *testing.T) {
	ctx := context.Background()
	orgA := newOrg(t, "org-A").OrgID
	orgB := newOrg(t, "org-B").OrgID

	dsA := newDatasetInOrg(t, orgA, "ds-A")
	dsB := newDatasetInOrg(t, orgB, "ds-B")

	aList, err := testStore.ListDatasets(ctx, orgA)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	if !containsDataset(aList, dsA) || containsDataset(aList, dsB) {
		t.Fatalf("org A 应只见 dsA(%d)，不见 dsB(%d)", dsA, dsB)
	}

	bList, err := testStore.ListDatasets(ctx, orgB)
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	if !containsDataset(bList, dsB) || containsDataset(bList, dsA) {
		t.Fatalf("org B 应只见 dsB(%d)，不见 dsA(%d)", dsB, dsA)
	}

	all, err := testStore.ListDatasets(ctx, nil) // 超管旁路
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if !containsDataset(all, dsA) || !containsDataset(all, dsB) {
		t.Fatal("超管应跨组织看到 dsA 与 dsB")
	}
}

// ④ accept-invite：用户落入邀请所属组织与角色；token 一次性；过期/邮箱不符被拒。
func TestAcceptInvite_JoinsCorrectOrgRole(t *testing.T) {
	ctx := context.Background()
	owner := newOrg(t, "org-invite")
	orgID := *owner.OrgID

	// 正常邀请（reviewer）。
	inv := newInvite(t, orgID, domain.RoleReviewer, "", owner.ID, time.Hour)
	email := uniqueEmail("invitee")
	u, err := testStore.AcceptInvite(ctx, inv.Token, "invitee", email, "x")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if u.OrgID == nil || *u.OrgID != orgID {
		t.Fatalf("受邀人应入组织 %d，got %v", orgID, u.OrgID)
	}
	if u.Role != domain.RoleReviewer {
		t.Fatalf("角色应取自邀请(reviewer)，got %s", u.Role)
	}

	// token 一次性：重复接受 → ErrNotFound。
	if _, err := testStore.AcceptInvite(ctx, inv.Token, "again", uniqueEmail("again"), "x"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("已用 token 应 ErrNotFound，got %v", err)
	}

	// 过期邀请被拒。
	expired := newInvite(t, orgID, domain.RoleAnnotator, "", owner.ID, -time.Hour)
	if _, err := testStore.AcceptInvite(ctx, expired.Token, "late", uniqueEmail("late"), "x"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("过期 token 应 ErrNotFound，got %v", err)
	}

	// 限定邮箱：不符 → ErrInviteEmailMismatch。
	restricted := newInvite(t, orgID, domain.RoleAnnotator, uniqueEmail("only"), owner.ID, time.Hour)
	if _, err := testStore.AcceptInvite(ctx, restricted.Token, "x", uniqueEmail("other"), "x"); !errors.Is(err, store.ErrInviteEmailMismatch) {
		t.Fatalf("邮箱不符应 ErrInviteEmailMismatch，got %v", err)
	}
}

// ---- helpers ----

func newDatasetInOrg(t *testing.T, orgID *int64, name string) int64 {
	t.Helper()
	id, err := testStore.CreateDataset(context.Background(), &domain.Dataset{
		OrgID: orgID, Name: name, SourceSchema: "s", SourceTable: "t", SourcePKColumn: "id",
		FormSchema:        json.RawMessage(`{"version":1,"columns":[]}`),
		FormSchemaVersion: 1, Status: domain.StatusReady,
	})
	if err != nil {
		t.Fatalf("create dataset %q: %v", name, err)
	}
	return id
}

func newInvite(t *testing.T, orgID int64, role domain.Role, email string, by int64, ttl time.Duration) *domain.Invite {
	t.Helper()
	tok := fmt.Sprintf("tok-%d", userSeq.Add(1))
	inv, err := testStore.CreateInvite(context.Background(), &domain.Invite{
		OrgID: orgID, Role: role, Token: tok, Email: email, CreatedBy: &by,
		ExpiresAt: time.Now().Add(ttl),
	})
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	return inv
}

func containsDataset(items []domain.DatasetListItem, id int64) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}
