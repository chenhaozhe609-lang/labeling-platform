package store

import (
	"context"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

// CreateOrgWithOwner 开放注册建组织：单事务内 insert org(owner_id NULL) → insert owner 用户(role=admin)
// → 回填 org.owner_id。解决 org.owner_id 与 user.org_id 互相引用的 chicken-egg。
// owner 的 OrgID/Role/IsSuperadmin 由本方法强制设定，调用方只需给 Username/Email/PasswordHash。
// 邮箱已存在 → ErrConflict（事务回滚）。
func (s *Store) CreateOrgWithOwner(ctx context.Context, orgName string, owner NewUser) (*domain.Organization, *domain.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)

	var org domain.Organization
	org.Name = orgName
	if err := tx.QueryRow(ctx,
		`INSERT INTO organizations (name) VALUES ($1) RETURNING id, created_at, updated_at`,
		orgName).Scan(&org.ID, &org.CreatedAt, &org.UpdatedAt); err != nil {
		return nil, nil, err
	}

	owner.OrgID = &org.ID
	owner.Role = domain.RoleAdmin
	owner.IsSuperadmin = false
	u, err := insertUser(ctx, tx, owner)
	if err != nil {
		return nil, nil, err // 含邮箱冲突 ErrConflict
	}

	if _, err := tx.Exec(ctx,
		`UPDATE organizations SET owner_id = $1, updated_at = now() WHERE id = $2`, u.ID, org.ID); err != nil {
		return nil, nil, err
	}
	org.OwnerID = &u.ID

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return &org, u, nil
}

func (s *Store) GetOrg(ctx context.Context, id int64) (*domain.Organization, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, owner_id, created_at, updated_at FROM organizations WHERE id = $1`, id)
	var o domain.Organization
	err := row.Scan(&o.ID, &o.Name, &o.OwnerID, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		return nil, mapNoRows(err)
	}
	return &o, nil
}

// ListOrgs 全部组织（超管运营/排障）。
func (s *Store) ListOrgs(ctx context.Context) ([]domain.Organization, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, owner_id, created_at, updated_at FROM organizations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Organization
	for rows.Next() {
		var o domain.Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.OwnerID, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
