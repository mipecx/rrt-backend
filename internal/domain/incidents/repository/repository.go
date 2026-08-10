// Package repository handles database interactions for incidents using Postgres and PostGIS.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mipecx/rrt_system/backend/internal/db"
	"github.com/mipecx/rrt_system/backend/internal/domain/incidents/model"
	"github.com/redis/go-redis/v9"
)

// ErrAlreadyAssigned возвращается, если инцидент уже занят другим экипажем
// (кто-то успел назначить раньше — гонка двух диспетчеров).
var ErrAlreadyAssigned = errors.New("incident is already assigned or not in a state that accepts assignment")

type Repository interface {
	Create(ctx context.Context, incident *model.Incident) error
	GetActive(ctx context.Context) ([]model.Incident, error)
	// AssignCrew назначает экипаж rrtID и диспетчера dispatcherID на инцидент incidentID,
	// переводя его статус в toStatus — но только если инцидент ещё ни на кого не назначен
	// (rrt_id IS NULL) и его текущий статус равен fromStatus. Если это не так —
	// возвращает ErrAlreadyAssigned. Проверка и запись атомарны (одно условное UPDATE).
	AssignCrew(ctx context.Context, incidentID, rrtID, dispatcherID uuid.UUID, fromStatus, toStatus model.IncidentStatus) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Incident, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status model.IncidentStatus) error
	ResolveIncident(ctx context.Context, id uuid.UUID) error
	UpdateLocation(ctx context.Context, incidentID uuid.UUID, lat, lng float64, battery *int32) error
}

type IncidentRepository struct {
	db  db.Querier
	rdb *redis.Client
}

func NewRepository(pool *pgxpool.Pool, rdb *redis.Client) Repository {
	return &IncidentRepository{
		db:  pool,
		rdb: rdb,
	}
}

// WithTx возвращает копию репозитория, работающую поверх переданной транзакции
// вместо пула — для использования внутри db.RunInTx.
func (r *IncidentRepository) WithTx(tx pgx.Tx) *IncidentRepository {
	return &IncidentRepository{db: tx, rdb: r.rdb}
}

func (r *IncidentRepository) Create(ctx context.Context, incident *model.Incident) error {
	// Если sector_id не был указан явно, пытаемся автоматически определить сектор по PostGIS полигонам
	if incident.SectorID == nil {
		var detectedSectorID uuid.UUID
		sectorQuery := `
			SELECT id FROM sectors 
			WHERE ST_Contains(area, ST_SetSRID(ST_MakePoint($1, $2), 4326)) 
			LIMIT 1;
		`
		err := r.db.QueryRow(ctx, sectorQuery, incident.Coords.Lng, incident.Coords.Lat).Scan(&detectedSectorID)
		if err == nil {
			incident.SectorID = &detectedSectorID
		}
	}

	wktCoords := fmt.Sprintf("POINT(%f %f)", incident.Coords.Lng, incident.Coords.Lat)

	query := `
		INSERT INTO incidents (
			id, tourist_id, rrt_id, dispatcher_id, type_id, sector_id, 
			status, battery, description, coords, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, ST_GeomFromText($10, 4326), $11, $12)
		RETURNING number;
	`

	err := r.db.QueryRow(
		ctx, query,
		incident.ID, incident.TouristID, incident.RRTID, incident.DispatcherID,
		incident.TypeID, incident.SectorID, incident.Status, incident.Battery,
		incident.Description, wktCoords, incident.CreatedAt, incident.UpdatedAt,
	).Scan(&incident.Number)
	if err != nil {
		return fmt.Errorf("failed to insert incident: %w", err)
	}

	return nil
}

type CachedLocation struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Battery *int32  `json:"battery,omitempty"`
}

func (r *IncidentRepository) GetActive(ctx context.Context) ([]model.Incident, error) {
	query := `
		SELECT 
			i.id, i.number, i.tourist_id, i.rrt_id, i.dispatcher_id, i.type_id, i.sector_id, 
			i.status, i.battery, i.description, ST_X(i.coords), ST_Y(i.coords), 
			i.created_at, i.updated_at, i.closed_at,
			u.fullname as tourist_name,
			COALESCE(t.name, 'Help request') as incident_type
		FROM incidents i
		JOIN users u ON i.tourist_id = u.id
		LEFT JOIN incident_types t ON i.type_id = t.id
		ORDER BY i.created_at DESC;
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active incidents: %w", err)
	}
	defer rows.Close()

	var incidents []model.Incident

	for rows.Next() {
		var inc model.Incident
		err = rows.Scan(
			&inc.ID, &inc.Number, &inc.TouristID, &inc.RRTID, &inc.DispatcherID,
			&inc.TypeID, &inc.SectorID, &inc.Status, &inc.Battery, &inc.Description,
			&inc.Coords.Lng, &inc.Coords.Lat, &inc.CreatedAt, &inc.UpdatedAt, &inc.ClosedAt,
			&inc.TouristName, &inc.IncidentType,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan incident row: %w", err)
		}

		// Если в Redis есть свежие оперативные GPS-координаты — обогащаем данные из RAM!
		if r.rdb != nil {
			val, err := r.rdb.Get(ctx, "geo:incident:"+inc.ID.String()).Result()
			if err == nil && val != "" {
				var loc CachedLocation
				if json.Unmarshal([]byte(val), &loc) == nil {
					inc.Coords.Lat = loc.Lat
					inc.Coords.Lng = loc.Lng
					if loc.Battery != nil {
						inc.Battery = loc.Battery
					}
				}
			}
		}

		incidents = append(incidents, inc)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return incidents, nil
}

func (r *IncidentRepository) AssignCrew(
	ctx context.Context,
	incidentID, rrtID, dispatcherID uuid.UUID,
	fromStatus, toStatus model.IncidentStatus,
) error {
	query := `
		UPDATE incidents
		SET rrt_id = $1, dispatcher_id = $2, status = $3, updated_at = now()
		WHERE id = $4 AND rrt_id IS NULL AND status = $5;
	`
	result, err := r.db.Exec(ctx, query, rrtID, dispatcherID, toStatus, incidentID, fromStatus)
	if err != nil {
		return fmt.Errorf("failed to assign crew to incident: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrAlreadyAssigned
	}

	return nil
}

func (r *IncidentRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Incident, error) {
	query := `
		SELECT 
			i.id, i.number, i.tourist_id, i.rrt_id, i.dispatcher_id, i.type_id, i.sector_id, 
			i.status, i.battery, i.description, ST_X(i.coords), ST_Y(i.coords), 
			i.created_at, i.updated_at, i.closed_at,
			u.fullname as tourist_name,
			COALESCE(t.name, 'Help request') as incident_type
		FROM incidents i
		JOIN users u ON i.tourist_id = u.id
		LEFT JOIN incident_types t ON i.type_id = t.id
		WHERE i.id = $1;
	`

	var inc model.Incident
	err := r.db.QueryRow(ctx, query, id).Scan(
		&inc.ID, &inc.Number, &inc.TouristID, &inc.RRTID, &inc.DispatcherID,
		&inc.TypeID, &inc.SectorID, &inc.Status, &inc.Battery, &inc.Description,
		&inc.Coords.Lng, &inc.Coords.Lat, &inc.CreatedAt, &inc.UpdatedAt, &inc.ClosedAt,
		&inc.TouristName, &inc.IncidentType,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get incident by id: %w", err)
	}

	return &inc, nil
}

func (r *IncidentRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.IncidentStatus) error {
	query := `
		UPDATE incidents
		SET status = $1, updated_at = now()
		WHERE id = $2;
	`
	_, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update incident status: %w", err)
	}
	return nil
}

func (r *IncidentRepository) ResolveIncident(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE incidents
		SET status = 'resolved', updated_at = now(), closed_at = now()
		WHERE id = $1;
	`
	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to resolve incident: %w", err)
	}
	return nil
}

func (r *IncidentRepository) UpdateLocation(ctx context.Context, incidentID uuid.UUID, lat, lng float64, battery *int32) error {
	// 1. Мгновенная запись оперативных GPS-координат в RAM Redis (субмиллисекундная задержка!)
	if r.rdb != nil {
		loc := CachedLocation{
			Lat:     lat,
			Lng:     lng,
			Battery: battery,
		}
		data, err := json.Marshal(loc)
		if err == nil {
			_ = r.rdb.Set(ctx, "geo:incident:"+incidentID.String(), string(data), 24*time.Hour).Err()
		}
	}

	// 2. Асинхронная фоновая синхронизация с PostgreSQL для PostGIS пространственных запросов
	go func() {
		asyncCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		query := `
			UPDATE incidents
			SET coords = ST_SetSRID(ST_MakePoint($1, $2), 4326),
			    updated_at = now(),
			    battery = COALESCE($3, battery)
			WHERE id = $4 AND status != 'resolved';
		`
		_, _ = r.db.Exec(asyncCtx, query, lng, lat, battery, incidentID)
	}()

	return nil
}
