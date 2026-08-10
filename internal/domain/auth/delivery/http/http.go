// Package http provides HTTP handlers for managing auth.
package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/mipecx/rrt_system/backend/internal/domain/auth/model"
	"github.com/mipecx/rrt_system/backend/internal/domain/auth/service"
)

type Handler struct {
	authService service.Service
	log         *slog.Logger
}

func NewHandler(authService service.Service, log *slog.Logger) *Handler {
	return &Handler{
		authService: authService,
		log:         log,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/send-otp", h.SendOTP)
	mux.HandleFunc("POST /api/v1/auth/register", h.Register)
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)
}

func (h *Handler) SendOTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("failed to decode send-otp request", "error", err)
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Phone == "" {
		h.respondWithError(w, http.StatusBadRequest, "phone is required")
		return
	}

	if err := h.authService.RequestOTP(r.Context(), req.Phone); err != nil {
		h.log.Error("failed to request otp", "phone", req.Phone, "error", err)
		h.respondWithError(w, http.StatusInternalServerError, "failed to send verification code")
		return
	}

	h.respondWithJSON(w, http.StatusOK, map[string]string{"status": "otp sent successfully"})
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("failed to decode register request", "error", err)
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Phone == "" || req.Password == "" || req.Code == "" {
		h.respondWithError(w, http.StatusBadRequest, "phone, password and otp code are required")
		return
	}

	tokens, err := h.authService.Register(r.Context(), req)
	if err != nil {
		h.log.Warn("registration failed", "phone", req.Phone, "error", err)
		h.respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondWithJSON(w, http.StatusCreated, tokens)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("failed to decode login request", "error", err)
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Phone == "" || req.Password == "" {
		h.respondWithError(w, http.StatusBadRequest, "phone and password are required")
		return
	}

	tokens, err := h.authService.Login(r.Context(), req)
	if err != nil {
		h.log.Warn("login failed", "phone", req.Phone, "error", err)
		h.respondWithError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	h.respondWithJSON(w, http.StatusOK, tokens)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("failed to decode refresh request", "error", err)
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		h.respondWithError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	tokens, err := h.authService.RefreshToken(r.Context(), req.RefreshToken)
	if err != nil {
		h.log.Warn("refresh failed", "error", err)
		h.respondWithError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	h.respondWithJSON(w, http.StatusOK, tokens)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Warn("failed to decode logout request", "error", err)
		h.respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		h.respondWithError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	if err := h.authService.Logout(r.Context(), req.RefreshToken); err != nil {
		h.log.Error("logout failed", "error", err)
		h.respondWithError(w, http.StatusInternalServerError, "failed to logout")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, map[string]string{"error": message})
}

func (h *Handler) respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.log.Error("failed to write json response", "error", err)
	}
}
