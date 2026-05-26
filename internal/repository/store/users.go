package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

// ListUsers 组织内全部用户（按 id 升序），供 admin 用户管理页。
// orgID 为 nil（超管）时返回全部用户。
func (s *Store) ListUsers(ctx context.Context, orgID *int64) ([]domain.User, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+userCols+` FROM users WHERE ($1::bigint IS NULL OR org_id = $1) ORDER BY id`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// CountAdmins 组织内 admin 数量（用于「保留至少一个 admin」守卫）。
// orgID 为 nil（超管）时统计全局。
func (s *Store) CountAdmins(ctx context.Context, orgID *int64) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE role = 'admin' AND ($1::bigint IS NULL OR org_id = $1)`, orgID).Scan(&n)
	return n, err
}

// UpdateUserRole 改角色。
func (s *Store) UpdateUserRole(ctx context.Context, id int64, role domain.Role) error {
	tag, err := s.pool.Exec(ctx, `UPDATE users SET role = $2, updated_at = now() WHERE id = $1`, id, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateUserPassword 重置密码（传入已 bcrypt 的 hash），同时 bump token_version——
// 改密即吊销该用户所有旧会话（旧 refresh 失效，旧 access 至多再活一个 TTL）。
func (s *Store) UpdateUserPassword(ctx context.Context, id int64, passwordHash string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, token_version = token_version + 1, updated_at = now() WHERE id = $1`,
		id, passwordHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// BumpTokenVersion +1 token_version，吊销该用户所有已签发 token（logout-all / 停用）。
func (s *Store) BumpTokenVersion(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET token_version = token_version + 1, updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser 删除用户。该用户已有标注/创建过数据集时无法删除，映射为 ErrConflict：
//   - 23503 foreign_key_violation：如 datasets.created_by（RESTRICT）。
//   - 23502 not_null_violation：annotations.user_id 为 NOT NULL，ON DELETE SET NULL 级联时反而触发。
func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		if isPgCode(err, "23503") || isPgCode(err, "23502") {
			return ErrConflict
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// isPgCode 判断错误是否为指定 SQLSTATE。
func isPgCode(err error, code string) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == code
}
