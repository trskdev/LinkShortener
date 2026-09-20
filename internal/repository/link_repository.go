package repository

import (
	"context"
	"errors"
	"fmt"
	"pht/pet/link_shortener/internal/domain"
	"pht/pet/link_shortener/internal/domain/db"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type PGXURLRepository struct {
	q *db.Queries
}

func NewPGXURLRepository(q *db.Queries) *PGXURLRepository {
	return &PGXURLRepository{q: q}
}

func (repo *PGXURLRepository) SaveLink(ctx context.Context, params db.CreateLinkParams) (*domain.Link, error) {
	linkRow, err := repo.q.CreateLink(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, domain.ErrCodeUniquenessConflict
		}
		return nil, fmt.Errorf("failed to save the link: %w", err)
	}

	link := &domain.Link{ID: linkRow.ID, ShortCode: linkRow.ShortCode, LongUrl: linkRow.LongUrl, Clicks: linkRow.Clicks}
	return link, nil
}

func (repo *PGXURLRepository) GetURLAndClicks(ctx context.Context, code string) (string, int32, error) {
	urlNClicks, err := repo.q.GetURLAndClicksByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", 0, domain.ErrLinkNotFound
		}
		return "", 0, fmt.Errorf("failed to get the link: %w", err)
	}

	return urlNClicks.LongUrl, urlNClicks.Clicks, nil
}

func (repo *PGXURLRepository) UpdateClicks(ctx context.Context, clicks int32, code string) error {
	updParams := db.UpdateClicksParams{ShortCode: code, Clicks: clicks}

	err := repo.q.UpdateClicks(ctx, updParams)
	if err != nil {
		return fmt.Errorf("failed to update clicks: %w", err)
	}

	return nil
}

func (repo *PGXURLRepository) IncrementClicks(ctx context.Context, code string) error {
	err := repo.q.IncrementClicks(ctx, code)
	if err != nil {
		return fmt.Errorf("failed to increment clicks: %w", err)
	}

	return nil
}
