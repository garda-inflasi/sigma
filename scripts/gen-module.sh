#!/usr/bin/env bash
# gen-module.sh — Generate a new Go module scaffold following Clean Architecture
# Usage: bash scripts/gen-module.sh <module_name>

set -euo pipefail

MODULE=$1
MODULE_DIR="apps/api/internal/${MODULE}"

if [ -z "$MODULE" ]; then
  echo "Usage: $0 <module_name>"
  exit 1
fi

if [ -d "$MODULE_DIR" ]; then
  echo "Error: Module $MODULE already exists at $MODULE_DIR"
  exit 1
fi

# Create directory structure
mkdir -p "${MODULE_DIR}/domain"
mkdir -p "${MODULE_DIR}/dto"
mkdir -p "${MODULE_DIR}/usecase"
mkdir -p "${MODULE_DIR}/delivery/http"
mkdir -p "${MODULE_DIR}/repository/postgres"
mkdir -p "${MODULE_DIR}/middleware"
mkdir -p "${MODULE_DIR}/routes"

# Create domain/entity.go
cat > "${MODULE_DIR}/domain/entity.go" << 'GOEOF'
package domain

import "time"

type Entity struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (Entity) TableName() string {
	return "entities"
}
GOEOF

# Create domain/repository.go
cat > "${MODULE_DIR}/domain/repository.go" << 'GOEOF'
package domain

import "context"

type Repository interface {
	// Define repository interface methods here
}
GOEOF

# Create dto/request.go
cat > "${MODULE_DIR}/dto/request.go" << 'GOEOF'
package dto

type CreateRequest struct {
	// Define request fields here
}

type UpdateRequest struct {
	// Define request fields here
}

type ListRequest struct {
	Page  int    `form:"page" binding:"min=1"`
	Limit int    `form:"limit" binding:"min=1,max=100"`
	Search string `form:"search"`
}
GOEOF

# Create dto/response.go
cat > "${MODULE_DIR}/dto/response.go" << 'GOEOF'
package dto

type ListResponse struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Limit int         `json:"limit"`
}
GOEOF

# Create usecase/usecase.go
cat > "${MODULE_DIR}/usecase/usecase.go" << 'GOEOF'
package usecase

type UseCase struct {
	repo domain.Repository
}

func New(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}
GOEOF

# Create delivery/http/handler.go
cat > "${MODULE_DIR}/delivery/http/handler.go" << 'GOEOF'
package http

type Handler struct {
	uc *usecase.UseCase
}

func New(uc *usecase.UseCase) *Handler {
	return &Handler{uc: uc}
}
GOEOF

# Create repository/postgres/repo.go
cat > "${MODULE_DIR}/repository/postgres/repo.go" << 'GOEOF'
package postgres

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}
GOEOF

# Create routes/routes.go
cat > "${MODULE_DIR}/routes/routes.go" << 'GOEOF'
package routes

import (
	"github.com/gin-gonic/gin"
)

type Options struct {
	// Module-specific options
}

func Register(engine *gin.RouterGroup, deps *Dependencies) {
	// Register routes here
}
GOEOF

echo "✅ Module '$MODULE' scaffolded at $MODULE_DIR"
echo ""
echo "Next steps:"
echo "  1. Edit domain/entity.go — define your entity"
echo "  2. Edit domain/repository.go — define repository interface"
echo "  3. Edit dto/request.go and dto/response.go — define DTOs"
echo "  4. Edit usecase/ — implement business logic"
echo "  5. Edit delivery/http/handler.go — implement handlers"
echo "  6. Edit repository/postgres/ — implement GORM adapter"
echo "  7. Edit routes/routes.go — register routes"
