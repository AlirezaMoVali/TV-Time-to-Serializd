package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alireza/tvtime2serializd/internal/tvtime"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MigrateJobStatus string

const (
	MigratePending   MigrateJobStatus = "pending"
	MigrateRunning   MigrateJobStatus = "running"
	MigrateCompleted MigrateJobStatus = "completed"
	MigrateFailed    MigrateJobStatus = "failed"
)

type MigrateJob struct {
	ID          uuid.UUID
	Status      MigrateJobStatus
	Progress    json.RawMessage
	DumpEnabled bool
	DumpFormat  *tvtime.OutputFormat
	ExportID    *uuid.UUID
	CreatedAt   time.Time
	CompletedAt *time.Time
}

type MigrateJobRepository struct {
	pool *pgxpool.Pool
}

func NewMigrateJobRepository(pool *pgxpool.Pool) *MigrateJobRepository {
	return &MigrateJobRepository{pool: pool}
}

func (r *MigrateJobRepository) Create(ctx context.Context, dumpEnabled bool, format *tvtime.OutputFormat) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO migrate_jobs (status, dump_enabled, dump_format)
		VALUES ('pending', $1, $2)
		RETURNING id
	`, dumpEnabled, format).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create migrate job: %w", err)
	}
	return id, nil
}

func (r *MigrateJobRepository) UpdateProgress(ctx context.Context, id uuid.UUID, status MigrateJobStatus, progress any) error {
	data, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE migrate_jobs SET status = $2, progress = $3 WHERE id = $1
	`, id, status, data)
	return err
}

func (r *MigrateJobRepository) MarkCompleted(ctx context.Context, id uuid.UUID, progress any, exportID *uuid.UUID) error {
	data, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE migrate_jobs
		SET status = 'completed', progress = $2, export_id = $3, completed_at = NOW()
		WHERE id = $1
	`, id, data, exportID)
	return err
}

func (r *MigrateJobRepository) MarkFailed(ctx context.Context, id uuid.UUID, progress any) error {
	data, err := json.Marshal(progress)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE migrate_jobs
		SET status = 'failed', progress = $2, completed_at = NOW()
		WHERE id = $1
	`, id, data)
	return err
}

func (r *MigrateJobRepository) GetByID(ctx context.Context, id uuid.UUID) (*MigrateJob, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, status, progress, dump_enabled, dump_format, export_id, created_at, completed_at
		FROM migrate_jobs WHERE id = $1
	`, id)

	var job MigrateJob
	var status string
	var format *string
	err := row.Scan(&job.ID, &status, &job.Progress, &job.DumpEnabled, &format, &job.ExportID, &job.CreatedAt, &job.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("get migrate job: %w", err)
	}
	job.Status = MigrateJobStatus(status)
	if format != nil {
		f := tvtime.OutputFormat(*format)
		job.DumpFormat = &f
	}
	return &job, nil
}
