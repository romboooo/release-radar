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
	IsNew      bool
}

func (s *Service) Delete(ctx context.Context, owner, repo string) error {
	return s.Store.DeleteRepo(ctx, owner, repo)
}
func (s *Service) ListUpdates(ctx context.Context) ([]store.UpdateRecord, error) {
	return s.Store.ListUpdates(ctx)
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
		result := CheckResult{Repository: repo}

		release, err := github.FetchLatestRelease(ctx, repo.Owner, repo.Repo)
		if err != nil {
			result.Err = err
			checks = append(checks, result)
			continue
		}

		result.Release = release
		record := store.ReleaseRecord{
			GitHubID:    release.ID,
			TagName:     release.TagName,
			URL:         release.HTMLURL,
			PublishedAt: release.PublishedAt,
		}

		result.IsNew, result.Err = s.Store.SaveRelease(ctx, repo.ID, record)
		checks = append(checks, result)
	}
	return checks, nil
}

func (s *Service) AddRepository(ctx context.Context, owner, repo string) error {
	return s.Store.AddRepository(ctx, owner, repo)
}
func (s *Service) ListRepositories(ctx context.Context) ([]store.Repository, error) {
	return s.Store.ListRepositories(ctx)
}
