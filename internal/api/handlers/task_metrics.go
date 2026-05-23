package handlers

import (
	"context"

	"github.com/Silo-Server/silo-server/internal/metadata"
	"github.com/Silo-Server/silo-server/internal/models"
)

type TaskMetricsService struct {
	refreshDebtRepo *metadata.RefreshDebtRepository
}

func NewTaskMetricsService(refreshDebtRepo *metadata.RefreshDebtRepository) *TaskMetricsService {
	return &TaskMetricsService{refreshDebtRepo: refreshDebtRepo}
}

func (s *TaskMetricsService) GetRefreshMetadataMetrics(ctx context.Context) (any, error) {
	if s == nil || s.refreshDebtRepo == nil {
		return &models.MetadataRefreshMetrics{}, nil
	}
	return s.refreshDebtRepo.GetMetrics(ctx, 10)
}
