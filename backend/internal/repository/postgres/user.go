package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/smartkrishi/backend/internal/domain"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

const userColumns = `
	id, name, email, phone_number, auth_provider, is_active, created_at, updated_at,
	auto_fallback_enabled, fallback_mode, fallback_active, fallback_phone,
	fallback_phone_verified, whatsapp_user_id
`

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE email = $1`
	row := r.pool.QueryRow(ctx, query, email)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return user, err
}

func (r *UserRepository) GetByID(ctx context.Context, id int32) (*domain.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return user, err
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE phone_number = $1`
	row := r.pool.QueryRow(ctx, query, phone)
	user, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return user, err
}

type CreateUserParams struct {
	Name             string
	Email            *string
	PhoneNumber      *string
	HashedPassword   *string
	AuthProvider     domain.AuthProvider
}

func (r *UserRepository) Create(ctx context.Context, params CreateUserParams) (*domain.User, error) {
	query := `
		INSERT INTO users (name, email, phone_number, hashed_password, auth_provider)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + userColumns

	row := r.pool.QueryRow(ctx, query,
		params.Name,
		params.Email,
		params.PhoneNumber,
		params.HashedPassword,
		string(params.AuthProvider),
	)
	return scanUser(row)
}

func (r *UserRepository) GetHashedPasswordByEmail(ctx context.Context, email string) (int32, string, bool, *string, error) {
	var id int32
	var hashedPassword *string
	var isActive bool
	var storedEmail *string

	err := r.pool.QueryRow(ctx, `
		SELECT id, hashed_password, is_active, email
		FROM users WHERE email = $1
	`, email).Scan(&id, &hashedPassword, &isActive, &storedEmail)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", false, nil, ErrUserNotFound
	}
	if err != nil {
		return 0, "", false, nil, err
	}
	if hashedPassword == nil {
		return id, "", isActive, storedEmail, nil
	}
	return id, *hashedPassword, isActive, storedEmail, nil
}

type scannable interface {
	Scan(dest ...any) error
}

func scanUser(row scannable) (*domain.User, error) {
	var user domain.User
	var authProvider string

	err := row.Scan(
		&user.ID,
		&user.Name,
		&user.Email,
		&user.PhoneNumber,
		&authProvider,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.AutoFallbackEnabled,
		&user.FallbackMode,
		&user.FallbackActive,
		&user.FallbackPhone,
		&user.FallbackPhoneVerified,
		&user.WhatsappUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}

	user.AuthProvider = domain.AuthProvider(authProvider)
	return &user, nil
}
