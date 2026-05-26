// Package store 是 meta-db 的 pgx 数据访问层（v1 不引 sqlc）。
package store

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chenhaozhe609-lang/labeling-platform/internal/domain"
)

// pgxQuerier 抽象 *pgxpool.Pool 与 pgx.Tx 的公共查询接口，
// 让 insertUser 等既能直接用连接池、也能在事务（signup / accept-invite）内复用。
type pgxQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

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

// email 可空（理论上仅超管引导前的占位场景），故 COALESCE 成空串，scanUser 一致按 string 扫。
const userCols = `id, username, COALESCE(email, '') AS email, password_hash, role, org_id, token_version, is_superadmin, created_at, updated_at`

// GetUserByEmail 按邮箱（大小写不敏感）查用户——登录标识。
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE lower(email) = lower($1)`, strings.TrimSpace(email))
	return scanUser(row)
}

func (s *Store) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	row := s.pool.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id)
	return scanUser(row)
}

// NewUser 是建用户的入参；OrgID 为 nil 且 IsSuperadmin 为 true 即平台超管。
type NewUser struct {
	Username     string
	Email        string
	PasswordHash string
	Role         domain.Role
	OrgID        *int64
	IsSuperadmin bool
}

// CreateUser 在连接池上建用户（邮箱冲突 → ErrConflict）。
func (s *Store) CreateUser(ctx context.Context, p NewUser) (*domain.User, error) {
	return insertUser(ctx, s.pool, p)
}

// insertUser 在任意 querier（连接池或事务）上插入用户。email 归一为小写去空格；
// 空 email 写 NULL（部分唯一索引允许多个 NULL）。
func insertUser(ctx context.Context, q pgxQuerier, p NewUser) (*domain.User, error) {
	var email any
	if e := strings.ToLower(strings.TrimSpace(p.Email)); e != "" {
		email = e
	}
	row := q.QueryRow(ctx,
		`INSERT INTO users (username, email, password_hash, role, org_id, is_superadmin)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+userCols,
		p.Username, email, p.PasswordHash, p.Role, p.OrgID, p.IsSuperadmin)
	u, err := scanUser(row)
	if isPgCode(err, "23505") { // unique_violation：邮箱已存在
		return nil, ErrConflict
	}
	return u, err
}

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role,
		&u.OrgID, &u.TokenVersion, &u.IsSuperadmin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
