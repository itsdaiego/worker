package repository

import (
	"context"
	"database/sql"
	. "challenge/model"

	"github.com/jmoiron/sqlx"
)

type JobRepository interface {
	GetAllJobs(queryOpts QueryOptions) ([]Job, error)
	GetJobById(id string) (Job, error)
	CreateJob(job Job) error
	BulkUpdateJobStatus(ids []string, status string) error
}

type QueryOptions struct {
	BatchSize int
}

type jobRepository struct {
	db *sqlx.DB
}

func NewJobRepository(db *sqlx.DB) JobRepository {
	return &jobRepository{db: db}
}

func (r *jobRepository) GetAllJobs(queryOpts QueryOptions) ([]Job, error) {
	var jobs []Job

	if queryOpts.BatchSize > 0 {
		tx, err := r.db.BeginTxx(context.Background(), &sql.TxOptions{})
		if err != nil {
			return nil, err
		}
		defer tx.Rollback()

		if err := tx.Select(&jobs, `
			SELECT id, type, payload, status
			FROM jobs
			WHERE status = $1
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		`, "pending", queryOpts.BatchSize); err != nil {
			return nil, err
		}
		if len(jobs) == 0 {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return jobs, nil
		}

		ids := make([]string, len(jobs))
		for i, j := range jobs {
			ids[i] = j.ID
		}

		query, args, err := sqlx.In("UPDATE jobs SET status = ? WHERE id IN (?)", "in_progress", ids)
		if err != nil {
			return nil, err
		}
		query = tx.Rebind(query)

		if _, err := tx.Exec(query, args...); err != nil {
			return nil, err
		}

		if err := tx.Commit(); err != nil {
			return nil, err
		}

		return jobs, nil
	}

	err := r.db.Select(&jobs, "SELECT id, type, payload, status FROM jobs")
	return jobs, err
}

func (r *jobRepository) GetJobById(id string) (Job, error) {
	var job Job

	err := r.db.Get(&job, "SELECT id, type, payload, status FROM jobs WHERE id = $1", id)
	return job, err
}

func (r *jobRepository) CreateJob(job Job) error {
	job.EnsureID()
	_, err := r.db.Exec(
		"INSERT INTO jobs (id, type, payload, status) VALUES ($1, $2, $3, $4)",
		job.ID,
		job.Type,
		job.Payload,
		job.Status,
	)
	return err
}

func (r *jobRepository) BulkUpdateJobStatus(ids []string, status string) error {
	if len(ids) == 0 {
		return nil
	}

	query, args, err := sqlx.In("UPDATE jobs SET status = ? WHERE id IN (?)", status, ids)
	if err != nil {
		return err
	}
	query = r.db.Rebind(query)
	_, err = r.db.Exec(query, args...)
	return err
}
