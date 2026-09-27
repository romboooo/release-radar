package tracker

import (
	"context"

	"github.com/romboooo/release-radar/internal/store"
)

type Service struct {
	Store *store.Store
}

func New(repoStore *store.Store) *Service {
	return &Service{Store: repoStore}
}

func (s *Service) AddRepository(ctx context.Context, owner, repo string) error {
	return s.Store.AddRepository(ctx, owner, repo)
}
