package tracker

import (
	"context"

	"github.com/romboooo/release-radar/internal/github"
	"github.com/romboooo/release-radar/internal/store"
)

type Service struct {
	Store *store.Store
}

type CheckResult struct {
	Repository store.Repository
	Release    github.Release
	Err        error
}

func New(repoStore *store.Store) *Service {
	return &Service{Store: repoStore}
}

func (s *Service) Check(ctx context.Context) ([]CheckResult, error) {
	repos, err := s.ListRepositories(ctx)

	if err != nil {
		return nil, err
	}

	var checks []CheckResult

	for _, repo := range repos {
		release, err := github.FetchLatestRelease(ctx, repo.Owner, repo.Repo)

		checks = append(checks, CheckResult{
			Repository: repo,
			Release:    release,
			Err:        err,
		})

	}
	return checks, nil
}

func (s *Service) AddRepository(ctx context.Context, owner, repo string) error {
	return s.Store.AddRepository(ctx, owner, repo)
}
func (s *Service) ListRepositories(ctx context.Context) ([]store.Repository, error) {
	return s.Store.ListRepositories(ctx)
}
