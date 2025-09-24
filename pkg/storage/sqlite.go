package storage

import (
	"database/sql"
	"fmt"

	"GoLinux/pkg/job"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	storage := &SQLiteStorage{db: db}

	if err := storage.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %v", err)
	}

	return storage, nil
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

func (s *SQLiteStorage) initSchema() error {
	query := `
	CREATE TABLE IF NOT EXISTS jobs (
		id TEXT PRIMARY KEY,
		command TEXT NOT NULL,
		status TEXT NOT NULL,
		result TEXT DEFAULT '',
		error_message TEXT DEFAULT '',
		created_at DATETIME NOT NULL,
		started_at DATETIME,
		completed_at DATETIME,
		exit_code INTEGER DEFAULT 0,
		timeout INTEGER NOT NULL,
		pid INTEGER DEFAULT -1
	);
	
	CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
	CREATE INDEX IF NOT EXISTS idx_jobs_created_at ON jobs(created_at);
	`

	_, err := s.db.Exec(query)
	return err
}

func (s *SQLiteStorage) SaveJob(j *job.Job) error {
	query := `
	INSERT OR REPLACE INTO jobs (
		id, command, status, result, error_message,
		created_at, started_at, completed_at, exit_code, timeout, pid
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var startedAt, completedAt interface{}
	if !j.StartedAt.IsZero() {
		startedAt = j.StartedAt
	}
	if !j.CompletedAt.IsZero() {
		completedAt = j.CompletedAt
	}

	_, err := s.db.Exec(query,
		j.ID, j.Command, string(j.Status), j.Result, j.ErrorMsg,
		j.CreatedAt, startedAt, completedAt, j.ExitCode, j.Timeout, j.Pid,
	)

	if err != nil {
		return fmt.Errorf("failed to save job %s: %v", j.ID, err)
	}

	return nil
}

func (s *SQLiteStorage) GetJob(jobID string) (*job.Job, error) {
	query := `
	SELECT id, command, status, result, error_message,
		   created_at, started_at, completed_at, exit_code, timeout, pid
	FROM jobs WHERE id = ?
	`

	row := s.db.QueryRow(query, jobID)

	j := &job.Job{}
	var startedAt, completedAt sql.NullTime

	err := row.Scan(
		&j.ID, &j.Command, &j.Status, &j.Result, &j.ErrorMsg,
		&j.CreatedAt, &startedAt, &completedAt, &j.ExitCode, &j.Timeout, &j.Pid,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("job with ID %s not found", jobID)
		}
		return nil, fmt.Errorf("failed to get job %s: %v", jobID, err)
	}

	// Handle nullable timestamps
	if startedAt.Valid {
		j.StartedAt = startedAt.Time
	}
	if completedAt.Valid {
		j.CompletedAt = completedAt.Time
	}

	return j, nil
}

func (s *SQLiteStorage) ListJobs(status job.Status) ([]*job.Job, error) {
	var query string
	var args []interface{}

	if status != "" {
		query = `
		SELECT id, command, status, result, error_message,
			   created_at, started_at, completed_at, exit_code, timeout, pid
		FROM jobs WHERE status = ? ORDER BY created_at DESC
		`
		args = append(args, string(status))
	} else {
		query = `
		SELECT id, command, status, result, error_message,
			   created_at, started_at, completed_at, exit_code, timeout, pid
		FROM jobs ORDER BY created_at DESC
		`
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %v", err)
	}
	defer rows.Close()

	var jobs []*job.Job

	for rows.Next() {
		j := &job.Job{}
		var startedAt, completedAt sql.NullTime

		err := rows.Scan(
			&j.ID, &j.Command, &j.Status, &j.Result, &j.ErrorMsg,
			&j.CreatedAt, &startedAt, &completedAt, &j.ExitCode, &j.Timeout, &j.Pid,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan job row: %v", err)
		}

		// Handle nullable timestamps
		if startedAt.Valid {
			j.StartedAt = startedAt.Time
		}
		if completedAt.Valid {
			j.CompletedAt = completedAt.Time
		}

		jobs = append(jobs, j)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating job rows: %v", err)
	}

	return jobs, nil
}

func (s *SQLiteStorage) DeleteAllJobs() (int64, error) {
	query := `DELETE FROM jobs`

	result, err := s.db.Exec(query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete all jobs: %v", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %v", err)
	}

	return rowsAffected, nil
}

func (s *SQLiteStorage) GetJobCount() (int, error) {
	query := `SELECT COUNT(*) FROM jobs`

	var count int
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get job count: %v", err)
	}

	return count, nil
}
