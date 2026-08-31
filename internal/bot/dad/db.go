package dad

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dadEntry struct {
	ID         int
	GuildID    string
	UserID     string
	Year       int
	Month      int
	IsOverride bool
	CreatedAt  time.Time
}

// Repository abstracts dad-of-month persistence. Command and scheduler logic
// depend on this interface so they can be tested against an in-memory fake;
// the concrete pgxRepository holds the real SQL and is never mocked.
type Repository interface {
	getDadForMonth(ctx context.Context, guildID string, year, month int) (*dadEntry, error)
	insertDad(ctx context.Context, guildID, userID string, year, month int, isOverride bool) error
	getHistory(ctx context.Context, guildID string) ([]dadRecord, error)
}

type pgxRepository struct {
	pool *pgxpool.Pool
}

// NewRepository returns the production Repository backed by Postgres.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgxRepository{pool: pool}
}

func (r *pgxRepository) getDadForMonth(ctx context.Context, guildID string, year, month int) (*dadEntry, error) {
	query := `
		SELECT "Id", "GuildId", "UserId", "Year", "Month", "IsOverride", "CreatedAt"
		FROM public.dad_of_month
		WHERE "GuildId" = $1 AND "Year" = $2 AND "Month" = $3
	`
	rows, err := r.pool.Query(ctx, query, guildID, year, month)
	if err != nil {
		return nil, err
	}

	entry, err := pgx.CollectOneRow(rows, pgx.RowToStructByNameLax[dadEntry])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &entry, nil
}

func (r *pgxRepository) insertDad(ctx context.Context, guildID, userID string, year, month int, isOverride bool) error {
	query := `
		INSERT INTO public.dad_of_month ("GuildId", "UserId", "Year", "Month", "IsOverride")
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT ("GuildId", "Year", "Month") DO UPDATE
			SET "UserId" = EXCLUDED."UserId",
			    "IsOverride" = EXCLUDED."IsOverride"
	`
	_, err := r.pool.Exec(ctx, query, guildID, userID, year, month, isOverride)
	return err
}

func (r *pgxRepository) getHistory(ctx context.Context, guildID string) ([]dadRecord, error) {
	query := `
		SELECT "UserId", "Year", "Month", "IsOverride"
		FROM public.dad_of_month
		WHERE "GuildId" = $1
		ORDER BY "Year" DESC, "Month" DESC
	`
	rows, err := r.pool.Query(ctx, query, guildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []dadRecord
	for rows.Next() {
		var rec dadRecord
		if err := rows.Scan(&rec.UserID, &rec.Year, &rec.Month, &rec.IsOverride); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
