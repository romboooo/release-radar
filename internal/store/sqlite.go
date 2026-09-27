package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Repository struct {
	Owner string
	Repo  string
}

func (s *Store) ListRepositories(ctx context.Context) ([]Repository, error) {
	rows, err := s.db.QueryContext(
		ctx,
		"select owner, repo from repositories order by owner,repo",
	)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}

	defer rows.Close()

	var repositories []Repository

	for rows.Next() {
		var r Repository
		if err := rows.Scan(&r.Owner, &r.Repo); err != nil {
			return nil, err
		}

		repositories = append(repositories, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return repositories, nil
}

func Open(path string) (*Store, error) {

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("db ping failed: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS repositories (
			id INTEGER PRIMARY KEY,
			owner TEXT NOT NULL,
			repo TEXT NOT NULL,
			UNIQUE(owner, repo)
		)
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create repositories table: %w", err)
	}

	return &Store{db: db}, nil
}
func (s *Store) AddRepository(ctx context.Context, owner, repo string) error {

	if len(owner) <= 0 {
		return fmt.Errorf(
			"empty owner")
	}

	if len(repo) <= 0 {
		return fmt.Errorf(
			"empty repo")
	}

	_, err := s.db.ExecContext(ctx, "insert into repositories (owner,repo) values (?,?)", owner, repo)

	if err != nil {
		return fmt.Errorf("error with adding repositories %w", err)
	}

	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
