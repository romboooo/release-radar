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
	ID    int64
	Owner string
	Repo  string
}

type ReleaseRecord struct {
	GitHubID    int64
	TagName     string
	URL         string
	PublishedAt string
}

type UpdateRecord struct {
	Owner        string
	Repo         string
	TagName      string
	URL          string
	PublishedAt  string
	DiscoveredAt string
}

func (s *Store) DeleteRepo(ctx context.Context, owner, repo string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("DeleteRepo: %w", err)
	}
	defer tx.Rollback()

	query1 := `DELETE FROM releases
		WHERE repository_id = (
			SELECT id FROM repositories WHERE owner = ? AND repo = ?
	);`
	query2 := `DELETE FROM repositories
		WHERE owner = ? AND repo = ?;`

	if _, err := tx.ExecContext(ctx, query1, owner, repo); err != nil {
		return fmt.Errorf("DeleteRepo: %w", err)
	}

	result, err := tx.ExecContext(ctx, query2, owner, repo)
	if err != nil {
		return fmt.Errorf("DeleteRepo: %w", err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeleteRepo: %w", err)
	}
	if affectedRows == 0 {
		return fmt.Errorf("DeleteRepo: there was no subscribes woth repo %s and owner %s", repo, owner)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("DeleteRepo: %w", err)
	}
	return nil
}

func (s *Store) ListUpdates(ctx context.Context) ([]UpdateRecord, error) {

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT p.owner, p.repo, r.tag_name, r.url, r.published_at, r.discovered_at
		FROM releases AS r
		JOIN repositories AS p ON p.id = r.repository_id
		ORDER BY r.discovered_at DESC, r.github_id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list updates: %w", err)
	}

	defer rows.Close()

	var records []UpdateRecord

	for rows.Next() {
		var rec UpdateRecord

		if err := rows.Scan(
			&rec.Owner,
			&rec.Repo,
			&rec.TagName,
			&rec.URL,
			&rec.PublishedAt,
			&rec.DiscoveredAt,
		); err != nil {
			return nil, fmt.Errorf("list updates: %w", err)
		}

		records = append(records, rec)

	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list updates: %w", err)
	}
	return records, nil
}

func (s *Store) SaveRelease(ctx context.Context, repositoryID int64, release ReleaseRecord) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO releases (repository_id, github_id, tag_name, url, published_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(repository_id, github_id) DO NOTHING
	`,
		repositoryID,
		release.GitHubID,
		release.TagName,
		release.URL,
		release.PublishedAt,
	)

	if err != nil {
		return false, fmt.Errorf("save release: %w", err)
	}
	affectedRows, err := result.RowsAffected()

	if err != nil {
		return false, fmt.Errorf("check inserted release: %w", err)
	}

	return affectedRows == 1, nil
}

func (s *Store) ListRepositories(ctx context.Context) ([]Repository, error) {
	rows, err := s.db.QueryContext(
		ctx,
		"select id, owner, repo from repositories order by owner,repo",
	)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}

	defer rows.Close()

	var repositories []Repository

	for rows.Next() {
		var r Repository
		if err := rows.Scan(&r.ID, &r.Owner, &r.Repo); err != nil {
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

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS releases (
			repository_id INTEGER NOT NULL,
			github_id INTEGER NOT NULL,
			tag_name TEXT NOT NULL,
			url TEXT NOT NULL,
			published_at TEXT NOT NULL,
			discovered_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (repository_id, github_id),
			FOREIGN KEY (repository_id) REFERENCES repositories(id)
			)
	`)

	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create releases table: %w", err)
	}

	return &Store{db: db}, nil
}
func (s *Store) AddRepository(ctx context.Context, owner, repo string) error {

	if len(owner) <= 0 {
		return fmt.Errorf("empty owner")
	}

	if len(repo) <= 0 {
		return fmt.Errorf("empty repo")
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
