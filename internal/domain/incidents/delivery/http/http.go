// Package http provides HTTP handlers for managing incidents.
package http

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/mipecx/rrt_system/backend/internal/domain/incidents/model"
	"github.com/mipecx/rrt_system/backend/internal/domain/incidents/service"
	"github.com/mipecx/rrt_system/backend/internal/middleware"
	"github.com/mipecx/rrt_system/backend/internal/ws"
)

type Handler struct {
	services       service.Service
	wsHub          *ws.Hub
	authMiddleware func(http.Handler) http.Handler
}

func NewHandler(services service.Service, wsHub *ws.Hub, authMiddleware func(http.Handler) http.Handler) *Handler {
	return &Handler{
		services:       services,
		wsHub:          wsHub,
		authMiddleware: authMiddleware,
	}
}

func (h *Handler) protect(roles ...string) func(http.HandlerFunc) http.Handler {
	return func(fn http.HandlerFunc) http.Handler {
		return h.authMiddleware(middleware.RequireRole(roles...)(fn))
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("POST /api/v1/incidents", h.protect("tourist")(h.Create))
	mux.Handle("GET /api/v1/incidents", h.protect("dispatcher", "rrt")(h.GetAllActive))

	mux.Handle("PUT /api/v1/incidents/{id}/assign", h.protect("dispatcher")(h.AssignRRT))
	mux.Handle("PUT /api/v1/incidents/{id}/arrive", h.protect("dispatcher", "rrt")(h.ArriveRRT))
	mux.Handle("PUT /api/v1/incidents/{id}/resolve", h.protect("dispatcher", "rrt")(h.Resolve))
	mux.Handle("PUT /api/v1/incidents/{id}/location", h.protect("tourist", "rrt")(h.UpdateLocation))
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TouristID == uuid.Nil {
		h.respondWithError(w, http.StatusBadRequest, "tourist_id is required")
		return
	}

	incident, err := h.services.CreateIncident(r.Context(), req)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.broadcastIncidentUpdate()

	h.respondWithJSON(w, http.StatusCreated, incident)
}

func (h *Handler) GetAllActive(w http.ResponseWriter, r *http.Request) {
	incidents, err := h.services.GetActiveIncidents(r.Context())
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if incidents == nil {
		incidents = []model.Incident{}
	}

	h.respondWithJSON(w, http.StatusOK, incidents)
}

func (h *Handler) AssignRRT(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	incidentID, err := uuid.Parse(idStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid incident uuid")
		return
	}

	type AssignRequest struct {
		RrtID uuid.UUID `json:"rrt_id"`
	}

	var req AssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RrtID == uuid.Nil {
		h.respondWithError(w, http.StatusBadRequest, "rrt_id is required")
		return
	}

	dispatcherID, err := uuid.Parse(middleware.GetUserID(r))
	if err != nil {
		h.respondWithError(w, http.StatusUnauthorized, "invalid dispatcher")
		return
	}

	err = h.services.AssignRRT(r.Context(), req.RrtID, incidentID, dispatcherID)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.broadcastRrtUpdate(req.RrtID, "en_route")
	h.broadcastIncidentUpdate()

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "rrt_assigned"})
}

func (h *Handler) ArriveRRT(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	incidentID, err := uuid.Parse(idStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid incident uuid")
		return
	}

	err = h.services.ArriveRRT(r.Context(), incidentID)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.broadcastIncidentUpdate()

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "rrt_arrived"})
}

func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	incidentID, err := uuid.Parse(idStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid incident uuid")
		return
	}

	err = h.services.ResolveIncident(r.Context(), incidentID)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.broadcastIncidentUpdate()

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "incident_resolved"})
}

type UpdateLocationRequest struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Battery *int32  `json:"battery,omitempty"`
}

func (h *Handler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	incidentID, err := uuid.Parse(idStr)
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid incident uuid")
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

	err = h.services.UpdateLocation(r.Context(), incidentID, req.Lat, req.Lng, req.Battery)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.wsHub != nil {
		msg := fmt.Sprintf(`{"type": "INCIDENT_UPDATE", "data": {"id": "%s", "lat": %f, "lng": %f}}`, incidentID, req.Lat, req.Lng)
		h.wsHub.Broadcast([]byte(msg))
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "location_updated"})
}

func (h *Handler) broadcastIncidentUpdate() {
	if h.wsHub != nil {
		msg := []byte(`{"type": "INCIDENT_UPDATE"}`)
		h.wsHub.Broadcast(msg)
	}
}

func (h *Handler) broadcastRrtUpdate(rrtID uuid.UUID, status string) {
	if h.wsHub != nil {
		msg := []byte(fmt.Sprintf(`{"type": "RRT_UPDATE", "data": {"id": "%s", "status": "%s"}}`, rrtID, status))
		h.wsHub.Broadcast(msg)
	}
}

func (h *Handler) respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *Handler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, map[string]string{"error": message})
}
