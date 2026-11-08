package repository

import (
	"context"
	"errors"
	"fmt"
	"leetx/url/shortener/domain"
	"leetx/url/shortener/utils"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type URLRepository struct {
	DB *pgxpool.Pool
}

func NewURLRepository(db *pgxpool.Pool) *URLRepository {
	return &URLRepository{DB: db}
}

func (repo *URLRepository) Create(ctx context.Context, shortCode, originalURL string,
	expiresAt *time.Time) error {
	query := `INSERT INTO urls (short_code, original_url, expires_at)VALUES ($1, $2, $3);`
	_, err := repo.DB.Exec(
		ctx,
		query,
		shortCode,
		originalURL,
		expiresAt,
	)

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return utils.ErrCodeExists
	}

	if err != nil {
		return fmt.Errorf("failed to create short URL: %w", err)
	}

	return nil
}

func (repo *URLRepository) GetByShortCode(ctx context.Context, shortCode string) (*domain.URL, error) {
	query := `SELECT original_url, expires_at FROM urls WHERE short_code = $1;`
	row := repo.DB.QueryRow(ctx, query, shortCode)
	var url domain.URL

	if err := row.Scan(&url.OriginalURL, &url.ExpiresAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, utils.ErrURLNotFound
		}

		return nil, fmt.Errorf("failed to retrieve original URL: %w", err)

	}

	return &url, nil
}

func (repo *URLRepository) Delete(ctx context.Context, shortCode string) error {
	query := `DELETE FROM urls WHERE short_code = $1;`
	result, err := repo.DB.Exec(ctx, query, shortCode)
	if err != nil {
		return fmt.Errorf("failed to delete short code: %w", err)
	}

	if result.RowsAffected() == 0 {
		return utils.ErrURLNotFound
	}

	return nil
}

func (repo *URLRepository) IncrementClickCount(ctx context.Context, shortCode string) error {
	query := `UPDATE urls SET click_count = click_count + 1 WHERE short_code = $1`
	result, err := repo.DB.Exec(ctx, query, shortCode)
	if err != nil {
		return fmt.Errorf("failed to update click count: %w", err)
	}

	if result.RowsAffected() == 0 {
		return utils.ErrURLNotFound
	}

	return nil
}

func (repo *URLRepository) IncrementClickCountBy(ctx context.Context, shortCode string, count int64) error {
	query := `UPDATE urls SET click_count = click_count + $1 WHERE short_code = $2`
	result, err := repo.DB.Exec(ctx, query, count, shortCode)
	if err != nil {
		return fmt.Errorf("failed to update click count: %w", err)
	}

	if result.RowsAffected() == 0 {
		return utils.ErrURLNotFound
	}

	return nil
}

func (repo *URLRepository) GetClickCount(ctx context.Context, shortCode string) (int64, error) {
	query := `SELECT click_count FROM urls WHERE short_code = $1`
	row := repo.DB.QueryRow(ctx, query, shortCode)

	var clickCount int64
	if err := row.Scan(&clickCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, utils.ErrURLNotFound
		}
		return 0, fmt.Errorf("failed to retrieve click count: %w", err)
	}

	return clickCount, nil
}
