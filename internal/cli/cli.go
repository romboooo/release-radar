package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/romboooo/release-radar/internal/tracker"
)

func Run(ctx context.Context, args []string, service *tracker.Service, out io.Writer) error {
	if len(args) != 2 || args[0] != "add" {
		return fmt.Errorf("usage: radar add owner/repo")
	}

	owner, repo, found := strings.Cut(args[1], "/")
	if !found || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return fmt.Errorf("repository must have the form owner/repo")
	}

	if err := service.AddRepository(ctx, owner, repo); err != nil {
		return err
	}

	_, err := fmt.Fprintf(out, "added repository: %s/%s\n", owner, repo)
	return err
}
