// Package service implements the business logic for managing Rapid Response Teams.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mipecx/rrt_system/backend/internal/db"
	"github.com/mipecx/rrt_system/backend/internal/domain/rrt/model"
	"github.com/mipecx/rrt_system/backend/internal/domain/rrt/repository"
	"golang.org/x/crypto/bcrypt"
)

type CreateRrtInput struct {
	Fullname string    `json:"fullname"`
	Phone    string    `json:"phone"`
	Password string    `json:"password"`
	SectorID uuid.UUID `json:"sector_id"`
}

type Service interface {
	CreateRrt(ctx context.Context, input CreateRrtInput) (*model.Rrt, error)
	GetCrews(ctx context.Context) ([]model.Rrt, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.RrtStatus) error
	UpdateLocation(ctx context.Context, id uuid.UUID, lat, lng float64) error
}

type RrtService struct {
	repo repository.Repository
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewService(repo repository.Repository, pool *pgxpool.Pool, log *slog.Logger) Service {
	return &RrtService{
		repo: repo,
		pool: pool,
		log:  log,
	}
}

func (s *RrtService) CreateRrt(ctx context.Context, input CreateRrtInput) (*model.Rrt, error) {
	if input.Fullname == "" {
		input.Fullname = "RRT Squad"
	}
	if input.Password == "" {
		input.Password = "password123"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	crewID := uuid.New()

	s.log.InfoContext(ctx, "creating new rrt crew with user account", "id", crewID, "phone", input.Phone, "fullname", input.Fullname)

	err = db.RunInTx(ctx, s.pool, func(tx pgx.Tx) error {
		txRepo := s.repo.(*repository.RrtRepository).WithTx(tx)
		return txRepo.CreateUserWithRrt(ctx, repository.CreateRrtUserParams{
			ID:           crewID,
			Phone:        input.Phone,
			PasswordHash: string(hash),
			Fullname:     input.Fullname,
			SectorID:     input.SectorID,
			Status:       model.StatusIdle,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("service.CreateRrt failed: %w", err)
	}

	transportType := "car"
	if strings.Contains(strings.ToLower(input.Fullname), "bike") || strings.Contains(strings.ToLower(input.Fullname), "motorcycle") {
		transportType = "motorcycle"
	}

	return &model.Rrt{
		ID:       crewID,
		Status:   model.StatusIdle,
		SectorID: input.SectorID,
		Name:     input.Fullname,
		Type:     transportType,
	}, nil
}

func (s *RrtService) GetCrews(ctx context.Context) ([]model.Rrt, error) {
	s.log.InfoContext(ctx, "fetching all rrt crews")

	crews, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("service.GetCrews failed: %w", err)
	}

	return crews, nil
}

func (s *RrtService) UpdateStatus(ctx context.Context, id uuid.UUID, status model.RrtStatus) error {
	switch status {
	case model.StatusOffline, model.StatusIdle, model.StatusEnRoute, model.StatusArrived, model.StatusBusy:
	default:
		return fmt.Errorf("invalid rrt status: %s", status)
	}

	s.log.InfoContext(ctx, "updating rrt crew status", "id", id, "new_status", status)

	if err := s.repo.ChangeStatus(ctx, id, status); err != nil {
		return fmt.Errorf("service.UpdateStatus failed: %w", err)
	}

	return nil
}

func (s *RrtService) UpdateLocation(ctx context.Context, id uuid.UUID, lat, lng float64) error {
	if err := s.repo.UpdateLocation(ctx, id, lat, lng); err != nil {
		return fmt.Errorf("service.UpdateLocation failed: %w", err)
	}
	return nil
}
