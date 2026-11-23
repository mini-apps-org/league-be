package permissions

import (
	"context"
	"log/slog"

	"github.com/nguyentanvinh/league-be/internal/platform/handler"
)

type Service interface {
	ListPermissions(ctx context.Context) ([]Permission, error)
	GetPermissionByID(ctx context.Context, id string) (*Permission, error)
	GetPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error)
}

type service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) Service {
	return &service{repo: repo, logger: logger.With(slog.String("component", "permissions.service"))}
}

func (s *service) ListPermissions(ctx context.Context) ([]Permission, error) {
	return s.repo.ListPermissions(ctx)
}

func (s *service) GetPermissionByID(ctx context.Context, id string) (*Permission, error) {
	permission, err := s.repo.GetPermissionByID(ctx, id)
	return permission, handler.MapFeatureError(err, permissionErrorMappings)
}

func (s *service) GetPermissionCodesByUserID(ctx context.Context, userID string) ([]string, error) {
	return s.repo.GetPermissionCodesByUserID(ctx, userID)
}
