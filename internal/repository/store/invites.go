package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

// ErrInviteEmailMismatch：邀请限定了受邀邮箱，但接受时给的邮箱与之不符。
var ErrInviteEmailMismatch = errors.New("invite email mismatch")

const inviteCols = `id, org_id, role, token, COALESCE(email, '') AS email, created_by,
	expires_at, accepted_at, accepted_by, created_at`

// CreateInvite 由管理者创建邀请（token / expires_at 由 handler 生成传入）。
func (s *Store) CreateInvite(ctx context.Context, inv *domain.Invite) (*domain.Invite, error) {
	var email any
	if e := strings.ToLower(strings.TrimSpace(inv.Email)); e != "" {
		email = e
	}
	row := s.pool.QueryRow(ctx,
		`INSERT INTO invites (org_id, role, token, email, created_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+inviteCols,
		inv.OrgID, inv.Role, inv.Token, email, inv.CreatedBy, inv.ExpiresAt)
	return scanInvite(row)
}

// ListInvites 组织内邀请（新到旧）。
func (s *Store) ListInvites(ctx context.Context, orgID int64) ([]domain.Invite, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+inviteCols+` FROM invites WHERE org_id = $1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Invite
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *inv)
	}
	return out, rows.Err()
}

// DeleteInvite 撤销邀请；限定本组织（防跨组织删别人的邀请）。
func (s *Store) DeleteInvite(ctx context.Context, orgID, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM invites WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AcceptInvite 凭 token 在邀请所属组织建用户（role 取自邀请），并标记邀请已用。单事务、行锁防并发重复接受。
//   - token 无效 / 已过期 / 已被使用 → ErrNotFound
//   - 邀请限定了邮箱且与给定 email 不符 → ErrInviteEmailMismatch
//   - email 已被占用 → ErrConflict
func (s *Store) AcceptInvite(ctx context.Context, token, username, email, passwordHash string) (*domain.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var (
		inviteID   int64
		orgID      int64
		role       domain.Role
		inviteMail string
	)
	err = tx.QueryRow(ctx,
		`SELECT id, org_id, role, COALESCE(email, '') FROM invites
		 WHERE token = $1 AND accepted_at IS NULL AND expires_at > now() FOR UPDATE`,
		token).Scan(&inviteID, &orgID, &role, &inviteMail)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if inviteMail != "" && !strings.EqualFold(strings.TrimSpace(inviteMail), strings.TrimSpace(email)) {
		return nil, ErrInviteEmailMismatch
	}

	u, err := insertUser(ctx, tx, NewUser{
		Username: username, Email: email, PasswordHash: passwordHash, Role: role, OrgID: &orgID,
	})
	if err != nil {
		return nil, err // 含邮箱冲突 ErrConflict
	}

	if _, err := tx.Exec(ctx,
		`UPDATE invites SET accepted_at = now(), accepted_by = $1 WHERE id = $2`, u.ID, inviteID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return u, nil
}

func scanInvite(row pgx.Row) (*domain.Invite, error) {
	var inv domain.Invite
	err := row.Scan(&inv.ID, &inv.OrgID, &inv.Role, &inv.Token, &inv.Email, &inv.CreatedBy,
		&inv.ExpiresAt, &inv.AcceptedAt, &inv.AcceptedBy, &inv.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}
