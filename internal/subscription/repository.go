package subscription

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewRepository(pool *pgxpool.Pool, log *slog.Logger) *Repository {
	return &Repository{pool: pool, log: log}
}

func (r *Repository) Create(ctx context.Context, item Subscription) (Subscription, error) {
	query := `
		INSERT INTO subscriptions (service_name, price, user_id, start_date, end_date)
		VALUES ($1, $2, $3::uuid, $4, $5)
		RETURNING id::text, service_name, price, user_id::text, start_date, end_date, created_at, updated_at
	`

	created, err := scanSubscription(r.pool.QueryRow(ctx, query,
		item.ServiceName,
		item.Price,
		item.UserID,
		item.StartDate,
		nullableTime(item.EndDate),
	))
	if err != nil {
		return Subscription{}, fmt.Errorf("create subscription: %w", mapPostgresError(err))
	}

	r.log.Info("subscription created", "id", created.ID, "user_id", created.UserID, "service_name", created.ServiceName)
	return created, nil
}

func (r *Repository) GetByID(ctx context.Context, id string) (Subscription, error) {
	query := `
		SELECT id::text, service_name, price, user_id::text, start_date, end_date, created_at, updated_at
		FROM subscriptions
		WHERE id = $1::uuid
	`

	item, err := scanSubscription(r.pool.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Subscription{}, ErrNotFound
	}
	if err != nil {
		return Subscription{}, fmt.Errorf("get subscription: %w", mapPostgresError(err))
	}
	return item, nil
}

func (r *Repository) List(ctx context.Context, filter ListFilter) ([]Subscription, error) {
	args := make([]any, 0, 4)
	conditions := []string{"1=1"}

	if filter.UserID != "" {
		args = append(args, filter.UserID)
		conditions = append(conditions, fmt.Sprintf("user_id = $%d::uuid", len(args)))
	}
	if filter.ServiceName != "" {
		args = append(args, filter.ServiceName)
		conditions = append(conditions, fmt.Sprintf("service_name = $%d", len(args)))
	}

	args = append(args, filter.Limit)
	limitPlaceholder := len(args)
	args = append(args, filter.Offset)
	offsetPlaceholder := len(args)

	query := fmt.Sprintf(`
		SELECT id::text, service_name, price, user_id::text, start_date, end_date, created_at, updated_at
		FROM subscriptions
		WHERE %s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, strings.Join(conditions, " AND "), limitPlaceholder, offsetPlaceholder)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", mapPostgresError(err))
	}
	defer rows.Close()

	items := make([]Subscription, 0)
	for rows.Next() {
		item, err := scanSubscription(rows)
		if err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate subscriptions: %w", err)
	}

	return items, nil
}

func (r *Repository) Update(ctx context.Context, id string, item Subscription) (Subscription, error) {
	query := `
		UPDATE subscriptions
		SET service_name = $2,
		    price = $3,
		    user_id = $4::uuid,
		    start_date = $5,
		    end_date = $6,
		    updated_at = now()
		WHERE id = $1::uuid
		RETURNING id::text, service_name, price, user_id::text, start_date, end_date, created_at, updated_at
	`

	updated, err := scanSubscription(r.pool.QueryRow(ctx, query,
		id,
		item.ServiceName,
		item.Price,
		item.UserID,
		item.StartDate,
		nullableTime(item.EndDate),
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return Subscription{}, ErrNotFound
	}
	if err != nil {
		return Subscription{}, fmt.Errorf("update subscription: %w", mapPostgresError(err))
	}

	r.log.Info("subscription updated", "id", updated.ID, "user_id", updated.UserID, "service_name", updated.ServiceName)
	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM subscriptions WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", mapPostgresError(err))
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}

	r.log.Info("subscription deleted", "id", id)
	return nil
}

func (r *Repository) CalculateTotal(ctx context.Context, filter TotalFilter) (int64, error) {
	args := []any{filter.PeriodStart, filter.PeriodEnd}
	conditions := []string{
		"start_date <= $2::date",
		"COALESCE(end_date, $2::date) >= $1::date",
	}

	if filter.UserID != "" {
		args = append(args, filter.UserID)
		conditions = append(conditions, fmt.Sprintf("user_id = $%d::uuid", len(args)))
	}
	if filter.ServiceName != "" {
		args = append(args, filter.ServiceName)
		conditions = append(conditions, fmt.Sprintf("service_name = $%d", len(args)))
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(
			price::bigint * (
				((EXTRACT(YEAR FROM LEAST(COALESCE(end_date, $2::date), $2::date))::int -
				  EXTRACT(YEAR FROM GREATEST(start_date, $1::date))::int) * 12)
				+ (EXTRACT(MONTH FROM LEAST(COALESCE(end_date, $2::date), $2::date))::int -
				   EXTRACT(MONTH FROM GREATEST(start_date, $1::date))::int)
				+ 1
			)
		), 0) AS total_price
		FROM subscriptions
		WHERE %s
	`, strings.Join(conditions, " AND "))

	var total int64
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("calculate total: %w", mapPostgresError(err))
	}

	return total, nil
}

func scanSubscription(row pgx.Row) (Subscription, error) {
	var item Subscription
	var startDate pgtype.Date
	var endDate pgtype.Date

	if err := row.Scan(
		&item.ID,
		&item.ServiceName,
		&item.Price,
		&item.UserID,
		&startDate,
		&endDate,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return Subscription{}, err
	}

	if startDate.Valid {
		item.StartDate = startDate.Time
	}
	if endDate.Valid {
		end := endDate.Time
		item.EndDate = &end
	}

	return item, nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func mapPostgresError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23514":
			return fmt.Errorf("check constraint violation: %s", pgErr.ConstraintName)
		case "23502":
			return fmt.Errorf("not-null constraint violation: %s", pgErr.ColumnName)
		case "22P02":
			return errors.New("invalid input syntax")
		}
	}
	return err
}
