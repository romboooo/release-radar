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

type HistoryRecord struct {
	Owner        string
	Repo         string
	Source       string
	TagName      string
	URL          string
	PublishedAt  string
	DiscoveredAt string
}

func (s *Store) DeleteRepo(ctx context.Context, owner, repo string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete repository: begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		DELETE FROM versions
		WHERE repository_id = (
			SELECT id FROM repositories WHERE owner = ? AND repo = ?
		)
	`, owner, repo)
	if err != nil {
		return fmt.Errorf("delete repository versions: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		DELETE FROM repositories
		WHERE owner = ? AND repo = ?
	`, owner, repo)
	if err != nil {
		return fmt.Errorf("delete repository: %w", err)
	}

	affectedRows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check deleted repository: %w", err)
	}
	if affectedRows == 0 {
		return fmt.Errorf("repository %s/%s is not tracked", owner, repo)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit repository deletion: %w", err)
	}
	return nil
}

func (s *Store) ListHistory(ctx context.Context) ([]HistoryRecord, error) {

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT p.owner, p.repo, v.source, v.tag_name, v.url,
			COALESCE(v.published_at, ''), v.discovered_at
		FROM versions AS v
		JOIN repositories AS p ON p.id = v.repository_id
		ORDER BY v.discovered_at DESC, v.source_id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}

	defer rows.Close()

	var records []HistoryRecord

	for rows.Next() {
		var rec HistoryRecord

		if err := rows.Scan(
			&rec.Owner,
			&rec.Repo,
			&rec.Source,
			&rec.TagName,
			&rec.URL,
			&rec.PublishedAt,
			&rec.DiscoveredAt,
		); err != nil {
			return nil, fmt.Errorf("list history: %w", err)
		}

		records = append(records, rec)

	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}
	return records, nil
}

func (s *Store) SaveTag(ctx context.Context, repositoryID int64, tagName string, tagUrl string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO versions (
			repository_id, source, source_id, tag_name, url, published_at
		)
		VALUES (?, 'tag', ?, ?, ?, NULL)
		ON CONFLICT(repository_id, source, source_id) DO NOTHING
	`,
		repositoryID,
		tagName,
		tagName,
		tagUrl,
	)
	if err != nil {
		return false, fmt.Errorf("SaveTag: %w", err)
	}

	affectedRows, err := result.RowsAffected()

	if err != nil {
		return false, fmt.Errorf("SaveTag: %w", err)
	}

	return affectedRows == 1, nil

}

func (s *Store) SaveRelease(ctx context.Context, repositoryID int64, release ReleaseRecord) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO versions (repository_id, source, source_id, tag_name, url, published_at)
		VALUES (?, 'release', ?, ?, ?, ?)
		ON CONFLICT(repository_id, source, source_id) DO NOTHING
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

	opened := false
	defer func() {
		if !opened {
			_ = db.Close()
		}
	}()

	ctx := context.Background()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin database setup: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS repositories (
			id INTEGER PRIMARY KEY,
			owner TEXT NOT NULL,
			repo TEXT NOT NULL,
			UNIQUE(owner, repo)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("create repositories table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS versions (
			repository_id INTEGER NOT NULL,
			source TEXT NOT NULL CHECK (source IN ('release', 'tag')),
			source_id TEXT NOT NULL,
			tag_name TEXT NOT NULL,
			url TEXT NOT NULL,
			published_at TEXT,
			discovered_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (repository_id, source, source_id),
			FOREIGN KEY (repository_id) REFERENCES repositories(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("create versions table: %w", err)
	}

	var hasLegacyReleases bool
	err = tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM sqlite_master
			WHERE type = 'table' AND name = 'releases'
		)
	`).Scan(&hasLegacyReleases)
	if err != nil {
		return nil, fmt.Errorf("check legacy releases table: %w", err)
	}

	if hasLegacyReleases {
		_, err = tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO versions (
				repository_id, source, source_id, tag_name,
				url, published_at, discovered_at
			)
			SELECT
				repository_id, 'release', CAST(github_id AS TEXT),
				tag_name, url, published_at, discovered_at
			FROM releases
		`)
		if err != nil {
			return nil, fmt.Errorf("migrate releases: %w", err)
		}

		var missing int
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM releases AS r
			WHERE NOT EXISTS (
				SELECT 1
				FROM versions AS v
				WHERE v.repository_id = r.repository_id
					AND v.source = 'release'
					AND v.source_id = CAST(r.github_id AS TEXT)
			)
		`).Scan(&missing)
		if err != nil {
			return nil, fmt.Errorf("verify migrated releases: %w", err)
		}
		if missing != 0 {
			return nil, fmt.Errorf("migration incomplete: %d releases missing", missing)
		}

		if _, err := tx.ExecContext(ctx, `DROP TABLE releases`); err != nil {
			return nil, fmt.Errorf("drop legacy releases table: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit database setup: %w", err)
	}

	opened = true
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
