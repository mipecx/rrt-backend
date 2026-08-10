// Package http provides HTTP handlers for managing RRT crews.
package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/mipecx/rrt_system/backend/internal/domain/rrt/model"
	"github.com/mipecx/rrt_system/backend/internal/domain/rrt/service"
	"github.com/mipecx/rrt_system/backend/internal/middleware"
	"github.com/mipecx/rrt_system/backend/internal/ws"
)

type Handler struct {
	services       service.Service
	wsHub          *ws.Hub
	log            *slog.Logger
	authMiddleware func(http.Handler) http.Handler
}

func NewHandler(services service.Service, wsHub *ws.Hub, log *slog.Logger, authMiddleware func(http.Handler) http.Handler) *Handler {
	return &Handler{
		services:       services,
		wsHub:          wsHub,
		log:            log,
		authMiddleware: authMiddleware,
	}
}

func (h *Handler) protect(roles ...string) func(http.HandlerFunc) http.Handler {
	return func(fn http.HandlerFunc) http.Handler {
		return h.authMiddleware(middleware.RequireRole(roles...)(fn))
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/rrt", h.protect("dispatcher")(h.Create))
	mux.Handle("GET /api/v1/rrt", h.protect("dispatcher")(h.GetAll))
	mux.Handle("PUT /api/v1/rrt/{id}/status", h.protect("dispatcher", "rrt")(h.UpdateStatus))
	mux.Handle("PUT /api/v1/rrt/{id}/location", h.protect("rrt")(h.UpdateLocation))
}

type CreateRrtRequest struct {
	Fullname string    `json:"fullname"`
	Phone    string    `json:"phone"`
	Password string    `json:"password"`
	SectorID uuid.UUID `json:"sector_id"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateRrtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SectorID == uuid.Nil {
		h.respondWithError(w, http.StatusBadRequest, "sector_id is required")
		return
	}

	crew, err := h.services.CreateRrt(r.Context(), service.CreateRrtInput{
		Fullname: req.Fullname,
		Phone:    req.Phone,
		Password: req.Password,
		SectorID: req.SectorID,
	})
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.wsHub != nil {
		h.wsHub.Broadcast([]byte(`{"type": "INCIDENT_UPDATE"}`))
	}

	h.respondWithJSON(w, http.StatusCreated, crew)
}

func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	crews, err := h.services.GetCrews(r.Context())
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusOK, crews)
}

type UpdateStatusRequest struct {
	Status model.RrtStatus `json:"status"`
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	crewID, err := uuid.Parse(idStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid rrt uuid")
		return
	}

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.services.UpdateStatus(r.Context(), crewID, req.Status); err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

type UpdateLocationRequest struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

func (h *Handler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	crewID, err := uuid.Parse(idStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid rrt uuid")
		return
	}

	var req UpdateLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Lat == 0 || req.Lng == 0 {
		h.respondWithError(w, http.StatusBadRequest, "lat and lng are required")
		return
	}

	if err := h.services.UpdateLocation(r.Context(), crewID, req.Lat, req.Lng); err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.wsHub != nil {
		msg := []byte(fmt.Sprintf(`{"type": "RRT_UPDATE", "data": {"id": "%s", "lat": %f, "lng": %f}}`, crewID, req.Lat, req.Lng))
		h.wsHub.Broadcast(msg)
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "location_updated"})
}

func (h *Handler) respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *Handler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, map[string]string{"error": message})
}
