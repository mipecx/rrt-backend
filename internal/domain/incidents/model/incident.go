// Package model contains the data structures and domain entities for incidents.
package model

import (
	"time"

	"github.com/google/uuid"
)

type IncidentStatus string

const (
	StatusCreated    IncidentStatus = "created"
	StatusInProgress IncidentStatus = "in_progress"
	StatusArrived    IncidentStatus = "arrived"
	StatusResolved   IncidentStatus = "resolved"
)

type Point struct {
	Lng float64 `json:"lng"`
	Lat float64 `json:"lat"`
}

type Incident struct {
	ID           uuid.UUID      `json:"id" db:"id"`
	Number       int32          `json:"number" db:"number"`
	TouristID    uuid.UUID      `json:"tourist_id" db:"tourist_id"`
	RRTID        *uuid.UUID     `json:"rrt_id" db:"rrt_id"`
	DispatcherID *uuid.UUID     `json:"dispatcher_id" db:"dispatcher_id"`
	TypeID       *uuid.UUID     `json:"type_id" db:"type_id"`
	SectorID     *uuid.UUID     `json:"sector_id" db:"sector_id"`
	Status       IncidentStatus `json:"status" db:"status"`
	Battery      *int32         `json:"battery" db:"battery"`
	Description  *string        `json:"description" db:"description"`
	Coords       Point          `json:"coords"`
	CreatedAt    time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at" db:"updated_at"`
	ClosedAt     *time.Time     `json:"closed_at" db:"closed_at"`

	TouristName  string `json:"tourist_name"`
	IncidentType string `json:"incident_type"`
}
type CreateIncidentRequest struct {
	TouristID   uuid.UUID `json:"tourist_id"`
	TypeID      uuid.UUID `json:"type_id"`
	Battery     int64     `json:"battery"`
	Description string    `json:"description"`
	Lng         float64   `json:"lng"`
	Lat         float64   `json:"lat"`
}
