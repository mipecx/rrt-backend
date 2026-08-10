// Package repository handles database operations for Rapid Response Teams (RRT).
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mipecx/rrt_system/backend/internal/db"
	"github.com/mipecx/rrt_system/backend/internal/domain/rrt/model"
)

// ErrStatusConflict возвращается, когда экипаж уже не в том статусе,
// из которого мы пытаемся его перевести (кто-то успел раньше).
var ErrStatusConflict = errors.New("rrt crew status conflict: not in expected state")

type CreateRrtUserParams struct {
	ID           uuid.UUID
	Phone        string
	PasswordHash string
	Fullname     string
	SectorID     uuid.UUID
	Status       model.RrtStatus
}

type Repository interface {
	Create(ctx context.Context, rrt *model.Rrt) error
	CreateUserWithRrt(ctx context.Context, params CreateRrtUserParams) error
	GetAll(ctx context.Context) ([]model.Rrt, error)
	ChangeStatus(ctx context.Context, id uuid.UUID, status model.RrtStatus) error
	// ChangeStatusConditional меняет статус экипажа с id только если его ТЕКУЩИЙ
	// статус равен from. Если это не так (экипаж уже занят/офлайн/etc — кто-то
	// успел раньше), возвращает ErrStatusConflict. Проверка и запись атомарны
	// (одно условное UPDATE), поэтому безопасно при конкурентных вызовах.
	ChangeStatusConditional(ctx context.Context, id uuid.UUID, from, to model.RrtStatus) error
	UpdateLocation(ctx context.Context, id uuid.UUID, lat, lng float64) error
}

type RrtRepository struct {
	db db.Querier
}

func NewRepository(pool *pgxpool.Pool) Repository {
	return &RrtRepository{
		db: pool,
	}
}

// WithTx возвращает копию репозитория, работающую поверх переданной транзакции
// вместо пула — для использования внутри db.RunInTx.
func (r *RrtRepository) WithTx(tx pgx.Tx) *RrtRepository {
	return &RrtRepository{db: tx}
}

func (r *RrtRepository) Create(ctx context.Context, rrt *model.Rrt) error {
	query := `
		INSERT INTO public.rrt (id, status, sector_id)
		VALUES ($1, $2, $3);
	`
	_, err := r.db.Exec(ctx, query, rrt.ID, rrt.Status, rrt.SectorID)
	if err != nil {
		return fmt.Errorf("failed to create rrt crew: %w", err)
	}
	return nil
}

func (r *RrtRepository) CreateUserWithRrt(ctx context.Context, params CreateRrtUserParams) error {
	userQuery := `
		INSERT INTO public.users (id, phone, password_hash, role, fullname)
		VALUES ($1, $2, $3, 'rrt', $4);
	`
	_, err := r.db.Exec(ctx, userQuery, params.ID, params.Phone, params.PasswordHash, params.Fullname)
	if err != nil {
		return fmt.Errorf("failed to insert rrt user: %w", err)
	}

	rrtQuery := `
		INSERT INTO public.rrt (id, status, sector_id)
		VALUES ($1, $2, $3);
	`
	_, err = r.db.Exec(ctx, rrtQuery, params.ID, params.Status, params.SectorID)
	if err != nil {
		return fmt.Errorf("failed to insert rrt profile: %w", err)
	}

	return nil
}

func (r *RrtRepository) GetAll(ctx context.Context) ([]model.Rrt, error) {
	query := `
		SELECT r.id, r.status, r.sector_id, u.fullname,
		       ST_X(r.coords), ST_Y(r.coords)
		FROM public.rrt r
		JOIN public.users u ON r.id = u.id;
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query rrt crews: %w", err)
	}
	defer rows.Close()

	// Инициализируем пустым слайсом вместо nil, чтобы фронт не поймал null в JSON
	crews := []model.Rrt{}

	for rows.Next() {
		var c model.Rrt
		var lng, lat *float64
		// Используем обычное '=', а не ':=' чтобы не затенять внешнюю err
		err = rows.Scan(&c.ID, &c.Status, &c.SectorID, &c.Name, &lng, &lat)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rrt row: %w", err)
		}
		if lng != nil && lat != nil {
			c.Coords = &model.Point{Lng: *lng, Lat: *lat}
		}
		c.Type = "car"
		if strings.Contains(strings.ToLower(c.Name), "bike") || strings.Contains(strings.ToLower(c.Name), "motorcycle") {
			c.Type = "motorcycle"
		}
		crews = append(crews, c)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in rrt query: %w", err)
	}

	return crews, nil
}

func (r *RrtRepository) ChangeStatus(ctx context.Context, id uuid.UUID, status model.RrtStatus) error {
	query := `
		UPDATE public.rrt 
		SET status = $1 
		WHERE id = $2;
	`
	result, err := r.db.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update rrt status: %w", err)
	}

	// Проверяем, обновилось ли хоть что-то (если пришел кривой id экипажа)
	if result.RowsAffected() == 0 {
		return fmt.Errorf("rrt crew with id %s not found", id)
	}

	return nil
}

func (r *RrtRepository) ChangeStatusConditional(ctx context.Context, id uuid.UUID, from, to model.RrtStatus) error {
	query := `
		UPDATE public.rrt
		SET status = $1
		WHERE id = $2 AND status = $3;
	`
	result, err := r.db.Exec(ctx, query, to, id, from)
	if err != nil {
		return fmt.Errorf("failed to conditionally update rrt status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrStatusConflict
	}

	return nil
}

func (r *RrtRepository) UpdateLocation(ctx context.Context, id uuid.UUID, lat, lng float64) error {
	query := `
		UPDATE public.rrt
		SET coords = ST_SetSRID(ST_MakePoint($1, $2), 4326),
		    updated_at = now()
		WHERE id = $3;
	`
	result, err := r.db.Exec(ctx, query, lng, lat, id)
	if err != nil {
		return fmt.Errorf("failed to update rrt location: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("rrt crew with id %s not found", id)
	}
	return nil
}
