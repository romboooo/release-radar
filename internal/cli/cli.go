package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/romboooo/release-radar/internal/tracker"
)

func runDelete(ctx context.Context, args []string, service *tracker.Service, out io.Writer) error {
	if len(args) != 2 {
		return fmt.Errorf("Usage: radar delete {owner}/{repo}")
	}

	owner, repo, found := strings.Cut(args[1], "/")
	if !found || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return fmt.Errorf("runDelete: repository must have the form owner/repo")
	}

	if err := service.Delete(ctx, owner, repo); err != nil {
		return fmt.Errorf("runDelete: %w", err)
	}

	if _, err := fmt.Fprintf(out, "repo %s/%s been deleted\n", owner, repo); err != nil {
		return fmt.Errorf("runDelete: %w", err)

	}

	return nil
}

func runUpdate(ctx context.Context, args []string, service *tracker.Service, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("Usage: radar update")
	}

	updates, err := service.ListUpdates(ctx)

	if err != nil {
		return err
	}

	for _, update := range updates {

		if _, err := fmt.Fprintf(out,
			"%s/%s %s %s %s\n",
			update.Owner,
			update.Repo,
			update.TagName,
			update.URL,
			update.PublishedAt,
		); err != nil {
			return err
		}
	}

	return nil
}

func runAdd(ctx context.Context, args []string, service *tracker.Service, out io.Writer) error {

	if len(args) != 2 {
		return fmt.Errorf("Usage: radar add {owner}/{repo}")
	}

	owner, repo, found := strings.Cut(args[1], "/")
	if !found || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return fmt.Errorf("repository must have the form owner/repo")
	}

	if err := service.AddRepository(ctx, owner, repo); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(out, "Added repository: %s/%s\n", owner, repo); err != nil {
		return err
	}
	return nil
}

func runList(ctx context.Context, args []string, service *tracker.Service, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("Usage: radar list")
	}
	repos, err := service.ListRepositories(ctx)
	if err != nil {
		return err
	}

	for _, repo := range repos {
		if _, err := fmt.Fprintf(out, "%s/%s\n", repo.Owner, repo.Repo); err != nil {
			return err
		}
	}

	return nil
}

func runCheck(ctx context.Context, args []string, service *tracker.Service, out io.Writer) error {

	if len(args) != 1 {
		return fmt.Errorf("Usage: radar check")
	}

	checkResults, err := service.Check(ctx)
	if err != nil {
		return err
	}

	for _, check := range checkResults {

		var err error
		status := "already seen"
		if check.IsNew {
			status = "new"
		}

		if check.Err != nil {
			_, err = fmt.Fprintf(
				out,
				"repo %s/%s error %v\n",
				check.Repository.Owner,
				check.Repository.Repo,
				check.Err,
			)

		} else {

			_, err = fmt.Fprintf(
				out,
				"%s/%s: %s (%s)\n",
				check.Repository.Owner,
				check.Repository.Repo,
				check.Release.TagName,
				status,
			)

		}

		if err != nil {
			return err
		}

	}

	return nil

}

func Run(ctx context.Context, args []string, service *tracker.Service, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("error: command should contain arguments")
	}
	if args[0] == "" {
		return fmt.Errorf("error: empty command name")
	}
	switch args[0] {
	case "add":
		return runAdd(ctx, args, service, out)
	case "list":
		return runList(ctx, args, service, out)
	case "check":
		return runCheck(ctx, args, service, out)
	case "update":
		return runUpdate(ctx, args, service, out)
	case "delete":
		return runDelete(ctx, args, service, out)
	default:
		return fmt.Errorf("error: invalid command name")
	}
}
