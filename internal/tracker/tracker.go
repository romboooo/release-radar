package tracker

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"

	"github.com/romboooo/release-radar/internal/github"
	"github.com/romboooo/release-radar/internal/store"
	"golang.org/x/mod/semver"
)

type Service struct {
	Store *store.Store
}

type CheckResult struct {
	Repository store.Repository
	Release    github.Release
	Err        error
	IsNew      bool
	Source     string
	Tag        github.Tag
}

func latestStableTag(tags []github.Tag) (github.Tag, error) {
	var latestStable github.Tag
	found := false

	for i := range tags {
		if !semver.IsValid(tags[i].Name) || semver.Prerelease(tags[i].Name) != "" {
			continue
		}

		if !found || semver.Compare(tags[i].Name, latestStable.Name) > 0 {
			latestStable = tags[i]
			found = true
		}

	}
	if !found {
		return github.Tag{}, fmt.Errorf("no stable version tags found")
	}
	return latestStable, nil
}

func (s *Service) Delete(ctx context.Context, owner, repo string) error {
	return s.Store.DeleteRepo(ctx, owner, repo)
}
func (s *Service) ListHistory(ctx context.Context) ([]store.HistoryRecord, error) {
	return s.Store.ListHistory(ctx)
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
			if err == nil {
				checks[i].Source = "release"
				checks[i].Release = release
				return
			}

			if errors.Is(err, github.ErrReleaseNotFound) {
				tags, err := github.FetchTags(ctx, repo.Owner, repo.Repo)

				if err != nil {
					checks[i].Err = err
					return
				}

				latestTag, err := latestStableTag(tags)

				if err != nil {
					checks[i].Err = err
					return
				}
				checks[i].Source = "tag"
				checks[i].Tag = latestTag

				return
			}

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

		switch checks[i].Source {
		case "tag":
			repo := checks[i].Repository
			tag := checks[i].Tag
			tagURL := fmt.Sprintf(
				"https://github.com/%s/%s/tree/%s",
				repo.Owner, repo.Repo, url.PathEscape(tag.Name),
			)
			checks[i].IsNew, checks[i].Err = s.Store.SaveTag(
				ctx, repo.ID, tag.Name, tagURL,
			)
		case "release":
			release := checks[i].Release

			checks[i].IsNew, checks[i].Err = s.Store.SaveRelease(ctx, checks[i].Repository.ID, store.ReleaseRecord{
				GitHubID:    release.ID,
				URL:         release.HTMLURL,
				TagName:     release.TagName,
				PublishedAt: release.PublishedAt,
			})
		}

	}

	return checks, nil
}

func (s *Service) AddRepository(ctx context.Context, owner, repo string) error {
	isRepoExists, err := github.RepositoryExists(ctx, owner, repo)

	if err != nil {
		return err
	}

	if !isRepoExists {
		return fmt.Errorf("repository %s/%s: not found", owner, repo)
	}

	return s.Store.AddRepository(ctx, owner, repo)
}
func (s *Service) ListRepositories(ctx context.Context) ([]store.Repository, error) {
	return s.Store.ListRepositories(ctx)
}
