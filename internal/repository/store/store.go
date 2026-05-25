// Package store 是 meta-db 的 pgx 数据访问层（v1 不引 sqlc）。
package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrNoTask    = errors.New("no pending task")     // claim 时池中无 PENDING
	ErrConflict  = errors.New("task state conflict") // submit/heartbeat 时任务已被回收或他人占用
	ErrForbidden = errors.New("forbidden")           // 越权：如审核本人提交的标注
)

// mapNoRows 把 pgx.ErrNoRows 统一转成 ErrNotFound。
func mapNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const userCols = `id, username, password_hash, role, created_at, updated_at`

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE username = $1`, username)
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string, role domain.Role) (*domain.User, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3) RETURNING `+userCols,
		username, passwordHash, role)
	u, err := scanUser(row)
	if isPgCode(err, "23505") { // unique_violation：用户名已存在
		return nil, ErrConflict
	}
	return u, err
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
