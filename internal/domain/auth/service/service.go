// Package service implements the core business logic for handling auth.
package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/mipecx/rrt_system/backend/internal/config"
	"github.com/mipecx/rrt_system/backend/internal/domain/auth/model"
	"github.com/mipecx/rrt_system/backend/internal/domain/auth/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service interface {
	RequestOTP(ctx context.Context, phone string) error
	VerifyOTP(ctx context.Context, phone, code string) (*model.TokenPair, error)
	RefreshToken(ctx context.Context, refreshToken string) (*model.TokenPair, error)
	Register(ctx context.Context, req model.RegisterRequest) (*model.TokenPair, error)
	Login(ctx context.Context, req model.LoginRequest) (*model.TokenPair, error)
	Logout(ctx context.Context, refreshToken string) error
}

type AuthService struct {
	repo repository.Repository
	cfg  config.JWTConfig
	log  *slog.Logger
}

func NewService(repo repository.Repository, cfg config.JWTConfig, log *slog.Logger) Service {
	return &AuthService{
		repo: repo,
		cfg:  cfg,
		log:  log,
	}
}

func (s *AuthService) RequestOTP(ctx context.Context, phone string) error {
	n, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return fmt.Errorf("failed to generate random numbers: %w", err)
	}
	code := fmt.Sprintf("%06d", n.Int64()+100000)

	if err := s.repo.SaveOTP(ctx, phone, code, 5*time.Minute); err != nil {
		return fmt.Errorf("failed to save OTP to repository: %w", err)
	}

	// TODO: Интеграция с SMS-шлюзом (Twilio / СМС-Центр) будет здесь

	s.log.Info("Successfully generated OTP",
		slog.String("phone", phone),
		slog.String("code", code),
	)

	return nil
}

func (s *AuthService) VerifyOTP(ctx context.Context, phone, code string) (*model.TokenPair, error) {
	savedCode, err := s.repo.GetOTP(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("failed to get OTP: %w", err)
	}

	if savedCode == "" || savedCode != code {
		return nil, errors.New("invalid or expired OTP")
	}

	_ = s.repo.DeleteOTP(ctx, phone)

	user, err := s.repo.GetByPhone(ctx, phone)
	if err != nil {
		return nil, fmt.Errorf("failed to check user: %w", err)
	}

	if user == nil {
		return &model.TokenPair{}, nil
	}

	tokens, err := s.generateTokenPair(ctx, user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("failed to manage session: %w", err)
	}

	s.log.Info("User logged in via OTP", "user_id", user.ID)
	return tokens, nil
}

func (s *AuthService) generateTokenPair(ctx context.Context, userID uuid.UUID, role string) (*model.TokenPair, error) {
	accessToken, err := generateAccessToken(userID, role, []byte(s.cfg.Secret), s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, err
	}

	refreshToken, err := generateRefreshToken(userID, []byte(s.cfg.Secret), s.cfg.RefreshTokenTTL)
	if err != nil {
		return nil, err
	}

	if err := s.repo.SaveRefreshToken(ctx, userID, refreshToken, s.cfg.RefreshTokenTTL); err != nil {
		return nil, fmt.Errorf("redis save: %w", err)
	}

	return &model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func generateAccessToken(userID uuid.UUID, role string, secret []byte, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"uuid": userID.String(),
		"role": role,
		"exp":  time.Now().Add(ttl).Unix(),
		"iat":  time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func generateRefreshToken(userID uuid.UUID, secret []byte, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"uuid": userID.String(),
		"exp":  time.Now().Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

func (s *AuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.TokenPair, error) {
	existingUser, err := s.repo.GetByPhone(ctx, req.Phone)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existingUser != nil {
		return nil, errors.New("user with this phone number already exists")
	}

	savedCode, err := s.repo.GetOTP(ctx, req.Phone)
	if err != nil {
		return nil, fmt.Errorf("failed to verify OTP during registration: %w", err)
	}
	if savedCode == "" || savedCode != req.Code {
		return nil, errors.New("invalid or expired confirmation code")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	now := time.Now()
	user := &model.User{
		ID:           uuid.New(),
		Phone:        req.Phone,
		Fullname:     req.FullName,
		PasswordHash: string(hash),
		Role:         model.Role(req.Role),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err = s.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to persist user: %w", err)
	}

	_ = s.repo.DeleteOTP(ctx, req.Phone)

	tokens, err := s.generateTokenPair(ctx, user.ID, string(user.Role))
	if err != nil {
		return nil, fmt.Errorf("failed to generate session after registration: %w", err)
	}

	s.log.Info("New user registered successfully", "user_id", user.ID, "role", user.Role)
	return tokens, nil
}

func (s *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.TokenPair, error) {
	user, err := s.repo.GetByPhone(ctx, req.Phone)
	if err != nil {
		return nil, fmt.Errorf("login failed: %w", err)
	}
	if user == nil {
		return nil, errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	s.log.Info("User logged in via password", "user_id", user.ID)
	return s.generateTokenPair(ctx, user.ID, string(user.Role))
}

func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (*model.TokenPair, error) {
	userID, err := s.repo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	if userID == uuid.Nil {
		return nil, errors.New("session expired or invalid")
	}

	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	_ = s.repo.DeleteRefreshToken(ctx, refreshToken)

	return s.generateTokenPair(ctx, user.ID, string(user.Role))
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if err := s.repo.DeleteRefreshToken(ctx, refreshToken); err != nil {
		return fmt.Errorf("failed to delete tokens on logout: %w", err)
	}
	return nil
}
