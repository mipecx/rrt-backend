// Package service implements the core business logic for handling incidents.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mipecx/rrt_system/backend/internal/db"
	"github.com/mipecx/rrt_system/backend/internal/domain/incidents/model"
	"github.com/mipecx/rrt_system/backend/internal/domain/incidents/repository"
	rrtModel "github.com/mipecx/rrt_system/backend/internal/domain/rrt/model"
	rrtRepo "github.com/mipecx/rrt_system/backend/internal/domain/rrt/repository"
)

type Service interface {
	CreateIncident(ctx context.Context, req model.CreateIncidentRequest) (*model.Incident, error)
	GetActiveIncidents(ctx context.Context) ([]model.Incident, error)
	AssignRRT(ctx context.Context, rrtID uuid.UUID, incidentID uuid.UUID, dispatcherID uuid.UUID) error
	ArriveRRT(ctx context.Context, incidentID uuid.UUID) error
	ResolveIncident(ctx context.Context, incidentID uuid.UUID) error
	UpdateLocation(ctx context.Context, incidentID uuid.UUID, lat, lng float64, battery *int32) error
}

type IncidentService struct {
	repo    repository.Repository
	rrtRepo rrtRepo.Repository
	pool    *pgxpool.Pool
}

func NewService(repo repository.Repository, rrtRepo rrtRepo.Repository, pool *pgxpool.Pool) Service {
	return &IncidentService{
		repo:    repo,
		rrtRepo: rrtRepo,
		pool:    pool,
	}
}

func (s *IncidentService) CreateIncident(ctx context.Context, req model.CreateIncidentRequest) (*model.Incident, error) {
	now := time.Now()

	incident := &model.Incident{
		ID:        uuid.New(),
		TouristID: req.TouristID,
		Status:    model.StatusCreated,
		Coords: model.Point{
			Lng: req.Lng,
			Lat: req.Lat,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	if req.TypeID != uuid.Nil {
		incident.TypeID = &req.TypeID
	}
	if req.Battery != 0 {
		b := int32(req.Battery)
		incident.Battery = &b
	}
	if req.Description != "" {
		incident.Description = &req.Description
	}

	if err := s.repo.Create(ctx, incident); err != nil {
		return nil, err
	}

	return incident, nil
}

func (s *IncidentService) GetActiveIncidents(ctx context.Context) ([]model.Incident, error) {
	return s.repo.GetActive(ctx)
}

func (s *IncidentService) AssignRRT(ctx context.Context, rrtID uuid.UUID, incidentID uuid.UUID, dispatcherID uuid.UUID) error {
	err := db.RunInTx(ctx, s.pool, func(tx pgx.Tx) error {
		txIncidentRepo := s.repo.(*repository.IncidentRepository).WithTx(tx)
		txRrtRepo := s.rrtRepo.(*rrtRepo.RrtRepository).WithTx(tx)

		// 1. Условно переводим экипаж в en_route из ready
		err := txRrtRepo.ChangeStatusConditional(ctx, rrtID, rrtModel.StatusIdle, rrtModel.StatusEnRoute)
		if err != nil {
			return fmt.Errorf("failed to change RRT status: %w", err)
		}

		// 2. Назначаем экипаж на инцидент
		err = txIncidentRepo.AssignCrew(ctx, incidentID, rrtID, dispatcherID, model.StatusCreated, model.StatusInProgress)
		if err != nil {
			return fmt.Errorf("failed to assign crew to incident: %w", err)
		}

		return nil
	})

	return err
}

func (s *IncidentService) ArriveRRT(ctx context.Context, incidentID uuid.UUID) error {
	incident, err := s.repo.GetByID(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("failed to get incident: %w", err)
	}
	if incident == nil {
		return fmt.Errorf("incident not found: %s", incidentID)
	}
	if incident.RRTID == nil {
		return fmt.Errorf("incident has no assigned RRT: %s", incidentID)
	}

	err = db.RunInTx(ctx, s.pool, func(tx pgx.Tx) error {
		txIncidentRepo := s.repo.(*repository.IncidentRepository).WithTx(tx)
		txRrtRepo := s.rrtRepo.(*rrtRepo.RrtRepository).WithTx(tx)

		// Переводим экипаж в arrived
		err := txRrtRepo.ChangeStatus(ctx, *incident.RRTID, rrtModel.StatusArrived)
		if err != nil {
			return fmt.Errorf("failed to update RRT status: %w", err)
		}

		// Ставим статус самого вызова "группа прибыла", чтобы турист видел прогресс
		if err := txIncidentRepo.UpdateStatus(ctx, incidentID, model.StatusArrived); err != nil {
			return fmt.Errorf("failed to mark incident arrived: %w", err)
		}

		return nil
	})

	return err
}

func (s *IncidentService) ResolveIncident(ctx context.Context, incidentID uuid.UUID) error {
	incident, err := s.repo.GetByID(ctx, incidentID)
	if err != nil {
		return fmt.Errorf("failed to get incident: %w", err)
	}
	if incident == nil {
		return fmt.Errorf("incident not found: %s", incidentID)
	}

	err = db.RunInTx(ctx, s.pool, func(tx pgx.Tx) error {
		txIncidentRepo := s.repo.(*repository.IncidentRepository).WithTx(tx)
		txRrtRepo := s.rrtRepo.(*rrtRepo.RrtRepository).WithTx(tx)

		// 1. Закрываем инцидент
		err := txIncidentRepo.ResolveIncident(ctx, incidentID)
		if err != nil {
			return fmt.Errorf("failed to resolve incident: %w", err)
		}

		// 2. Освобождаем экипаж RRT, если он был назначен
		if incident.RRTID != nil {
			err = txRrtRepo.ChangeStatus(ctx, *incident.RRTID, rrtModel.StatusIdle)
			if err != nil {
				return fmt.Errorf("failed to free RRT crew: %w", err)
			}
		}

		return nil
	})

	return err
}

func (s *IncidentService) UpdateLocation(ctx context.Context, incidentID uuid.UUID, lat, lng float64, battery *int32) error {
	return s.repo.UpdateLocation(ctx, incidentID, lat, lng, battery)
}
