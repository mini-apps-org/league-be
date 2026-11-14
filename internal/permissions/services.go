package permissions

import (
	"context"
	"errors"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

type Service interface {
	ListPermissions(ctx context.Context) ([]Permission, error)
	GetPermissionByID(ctx context.Context, id string) (*Permission, error)
	GetPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) ListPermissions(ctx context.Context) ([]Permission, error) {
	return s.repo.ListPermissions(ctx)
}

func (s *service) GetPermissionByID(ctx context.Context, id string) (*Permission, error) {
	permission, err := s.repo.GetPermissionByID(ctx, id)
	if errors.Is(err, ErrPermissionNotFound) {
		return nil, handler.Error(handler.CodeNotFound, err)
	}
	return permission, err
}

func (s *service) GetPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error) {
	return s.repo.GetPermissionCodesByUserID(ctx, userID)
}
