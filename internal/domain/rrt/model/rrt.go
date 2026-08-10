// Package model contains the data structures and domain entities for RRT crews.
package model

import (
	"github.com/google/uuid"
)

type RrtStatus string

const (
	StatusOffline RrtStatus = "offline"
	StatusIdle    RrtStatus = "ready"
	StatusEnRoute RrtStatus = "en_route"
	StatusArrived RrtStatus = "arrived"
	StatusBusy    RrtStatus = "busy"
)

type Rrt struct {
	ID       uuid.UUID `json:"id"`
	Status   RrtStatus `json:"status"`
	SectorID uuid.UUID `json:"sector_id"`
	Name     string    `json:"name,omitempty"`
	Type     string    `json:"type,omitempty"`
	Coords   *Point    `json:"coords,omitempty"`
}

type Point struct {
	Lng float64 `json:"lng"`
	Lat float64 `json:"lat"`
}
