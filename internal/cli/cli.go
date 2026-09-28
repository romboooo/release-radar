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

func runHistory(ctx context.Context, args []string, service *tracker.Service, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("Usage: radar history")
	}

	history, err := service.ListHistory(ctx)

	if err != nil {
		return err
	}

	for _, record := range history {

		if _, err := fmt.Fprintf(out,
			"%s/%s %s %s %s %s\n",
			record.Owner,
			record.Repo,
			record.TagName,
			record.Source,
			record.URL,
			record.PublishedAt,
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
			version := check.Release.TagName

			if check.Source == "tag" {
				version = check.Tag.Name
			}

			_, err = fmt.Fprintf(
				out,
				"%s/%s: %s [%s] (%s)\n",
				check.Repository.Owner,
				check.Repository.Repo,
				version,
				check.Source,
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
	case "history":
		return runHistory(ctx, args, service, out)
	case "delete":
		return runDelete(ctx, args, service, out)
	default:
		return fmt.Errorf("error: invalid command name")
	}
}
