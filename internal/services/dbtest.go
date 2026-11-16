package services

import "context"

type DBTestRepository interface {
	SaveMessage(ctx context.Context, message string) error
}

type DBTestService struct {
	repository DBTestRepository
}

func NewDBTestService(repository DBTestRepository) *DBTestService {
	return &DBTestService{repository: repository}
}

func (s *DBTestService) SaveMessage(ctx context.Context, message string) error {
	return s.repository.SaveMessage(ctx, message)
}
