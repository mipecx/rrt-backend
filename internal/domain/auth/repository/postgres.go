// Package repository handles database interactions for auth using Postgres and Redis.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/mipecx/rrt_system/backend/internal/domain/auth/model"
)

type Repository interface {
	GetOTP(ctx context.Context, phone string) (string, error)
	SaveOTP(ctx context.Context, phone, code string, ttl time.Duration) error
	DeleteOTP(ctx context.Context, phone string) error
	GetRefreshToken(ctx context.Context, token string) (uuid.UUID, error)
	SaveRefreshToken(ctx context.Context, userID uuid.UUID, token string, ttl time.Duration) error
	DeleteRefreshToken(ctx context.Context, token string) error
	CreateUser(ctx context.Context, u *model.User) error
	GetByPhone(ctx context.Context, phone string) (*model.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
}

type AuthRepository struct {
	pg  *pgxpool.Pool
	rdb *redis.Client
}

func NewRepository(pg *pgxpool.Pool, rdb *redis.Client) Repository {
	return &AuthRepository{
		pg:  pg,
		rdb: rdb,
	}
}

func (r *AuthRepository) GetOTP(ctx context.Context, phone string) (string, error) {
	key := "otp:" + phone
	code, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	return code, err
}

func (r *AuthRepository) SaveOTP(ctx context.Context, phone, code string, ttl time.Duration) error {
	key := "otp:" + phone
	return r.rdb.Set(ctx, key, code, ttl).Err()
}

func (r *AuthRepository) DeleteOTP(ctx context.Context, phone string) error {
	key := "otp:" + phone
	return r.rdb.Del(ctx, key).Err()
}

func (r *AuthRepository) GetRefreshToken(ctx context.Context, token string) (uuid.UUID, error) {
	key := "refresh:" + token
	userIDStr, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return uuid.Nil, nil
		}
		return uuid.Nil, err
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse uuid from redis: %w", err)
	}

	return userID, nil
}

func (r *AuthRepository) SaveRefreshToken(ctx context.Context, userID uuid.UUID, token string, ttl time.Duration) error {
	key := "refresh:" + token
	return r.rdb.Set(ctx, key, userID.String(), ttl).Err()
}

func (r *AuthRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	key := "refresh:" + token
	return r.rdb.Del(ctx, key).Err()
}

func (r *AuthRepository) CreateUser(ctx context.Context, u *model.User) error {
	query := `
		INSERT INTO users (id, phone, password_hash, role, fullname, avatar_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.pg.Exec(ctx, query, u.ID,
		u.Phone,
		u.PasswordHash,
		u.Role,
		u.Fullname,
		u.AvatarURL,
		u.CreatedAt,
		u.UpdatedAt,
	)
	return err
}

func (r *AuthRepository) GetByPhone(ctx context.Context, phone string) (*model.User, error) {
	query := `
		SELECT id, phone, password_hash, role, fullname, avatar_url, created_at, updated_at
		FROM users
		WHERE phone = $1
	`
	var u model.User
	err := r.pg.QueryRow(ctx, query, phone).Scan(
		&u.ID,
		&u.Phone,
		&u.PasswordHash,
		&u.Role,
		&u.Fullname,
		&u.AvatarURL,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (r *AuthRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	query := `
		SELECT id, phone, password_hash, role, fullname, avatar_url, created_at, updated_at 
		FROM users 
		WHERE id = $1
	`
	var u model.User
	err := r.pg.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Phone,
		&u.PasswordHash,
		&u.Role,
		&u.Fullname,
		&u.AvatarURL,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}
