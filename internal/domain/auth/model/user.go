// Package model contains the data structures and domain entities for auth.
package model

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleTourist    Role = "tourist"
	RoleDispatcher Role = "dispatcher"
	RoleRRT        Role = "rrt"
)

// TODO: Strip the model and create DTO

type User struct {
	ID           uuid.UUID `json:"id"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	Fullname     string    `json:"fullname"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type RegisterRequest struct {
	Phone    string `json:"phone" validate:"required"`
	Code     string `json:"code" validate:"required,len=6"`
	Password string `json:"password" validate:"required,min=6"`
	FullName string `json:"full_name" validate:"required"`
	Role     string `json:"role" validate:"required,oneof=tourist rrt dispatcher"`
}

type LoginRequest struct {
	Phone    string `json:"phone" validate:"required"`
	Password string `json:"password" validate:"required"`
}
