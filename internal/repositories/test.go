package repositories

import "context"

type TestRepository struct{}

func NewTestRepository() *TestRepository {
	return &TestRepository{}
}

func (r *TestRepository) Message(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	return "Hello!", nil
}
