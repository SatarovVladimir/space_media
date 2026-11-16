package services

import "context"

type TestRepository interface {
	Message(ctx context.Context) (string, error)
}

type TestService struct {
	repository TestRepository
}

func NewTestService(repository TestRepository) *TestService {
	return &TestService{repository: repository}
}

func (s *TestService) Message(ctx context.Context) (string, error) {
	return s.repository.Message(ctx)
}
