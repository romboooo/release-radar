package tracker

import (
	"context"
	"sync"

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

	checks := make([]CheckResult, len(repos))

	var wg sync.WaitGroup
	slots := make(chan struct{}, 3)

	for i, repo := range repos {
		checks[i].Repository = repo

		wg.Add(1)

		go func(i int, repo store.Repository) {
			defer wg.Done()

			select {
			case slots <- struct{}{}:
			case <-ctx.Done():
				checks[i].Err = ctx.Err()
				return
			}

			defer func() {
				<-slots
			}()

			release, err := github.FetchLatestRelease(ctx, repo.Owner, repo.Repo)
			checks[i].Release = release
			checks[i].Err = err

		}(i, repo)
	}

	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i := range checks {

		if checks[i].Err != nil {
			continue
		}

		release := checks[i].Release

		checks[i].IsNew, checks[i].Err = s.Store.SaveRelease(ctx, checks[i].Repository.ID, store.ReleaseRecord{
			GitHubID:    release.ID,
			URL:         release.HTMLURL,
			TagName:     release.TagName,
			PublishedAt: release.PublishedAt,
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
