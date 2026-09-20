//go:build integration

package repository

import (
	"context"
	"pht/pet/link_shortener/internal/domain"
	"pht/pet/link_shortener/internal/domain/db"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkRepo_Create_Success(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	queries := db.New(pool)
	repo := NewPGXURLRepository(queries)

	_, err := repo.SaveLink(ctx, db.CreateLinkParams{
		ShortCode: "first",
		LongUrl:   "https://example.com",
	})
	require.NoError(t, err)
}

func TestLinkRepo_Create_Same_URL_Success(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	queries := db.New(pool)
	repo := NewPGXURLRepository(queries)

	_, err := repo.SaveLink(ctx, db.CreateLinkParams{
		ShortCode: "first",
		LongUrl:   "https://example.com",
	})
	require.NoError(t, err)

	_, err = repo.SaveLink(ctx, db.CreateLinkParams{
		ShortCode: "second",
		LongUrl:   "https://example.com",
	})
	require.NoError(t, err)
}

func TestLinkRepo_Create_DuplicateErr(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	queries := db.New(pool)
	repo := NewPGXURLRepository(queries)

	_, err := repo.SaveLink(ctx, db.CreateLinkParams{
		ShortCode: "first",
		LongUrl:   "https://example.com",
	})
	require.NoError(t, err)

	_, err = repo.SaveLink(ctx, db.CreateLinkParams{
		ShortCode: "first",
		LongUrl:   "https://example.com",
	})
	assert.ErrorIs(t, err, domain.ErrCodeUniquenessConflict, "Error of uniqueness conflict expected")
}

func TestLinkRepo_Get_Success(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	queries := db.New(pool)
	repo := NewPGXURLRepository(queries)

	created, _ := repo.SaveLink(ctx, db.CreateLinkParams{
		ShortCode: "first",
		LongUrl:   "https://example.com",
	})

	url, clicks, err := repo.GetURLAndClicks(ctx, "first")
	require.NoError(t, err)
	require.Equal(t, created.LongUrl, url)
	require.Equal(t, int32(0), clicks)
}

func TestLinkRepo_Get_NotFoundErr(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	queries := db.New(pool)
	repo := NewPGXURLRepository(queries)

	_, _, err := repo.GetURLAndClicks(ctx, "first")
	assert.ErrorIs(t, err, domain.ErrLinkNotFound, "Error not found expected")
}

func TestLinkRepo_Update_Clicks_Success(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	queries := db.New(pool)
	repo := NewPGXURLRepository(queries)

	_, _ = repo.SaveLink(ctx, db.CreateLinkParams{
		ShortCode: "first",
		LongUrl:   "https://example.com",
	})

	_, clicks, _ := repo.GetURLAndClicks(ctx, "first")
	require.Equal(t, int32(0), clicks)

	err := repo.UpdateClicks(ctx, 1, "first")
	require.NoError(t, err)

	_, clicks, _ = repo.GetURLAndClicks(ctx, "first")
	require.Equal(t, int32(1), clicks)

	err = repo.UpdateClicks(ctx, 5, "first")
	require.NoError(t, err)

	_, clicks, _ = repo.GetURLAndClicks(ctx, "first")
	require.Equal(t, int32(6), clicks)
}

func TestLinkRepo_Increment_Clicks_Success(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	queries := db.New(pool)
	repo := NewPGXURLRepository(queries)

	_, _ = repo.SaveLink(ctx, db.CreateLinkParams{
		ShortCode: "first",
		LongUrl:   "https://example.com",
	})

	_, clicks, _ := repo.GetURLAndClicks(ctx, "first")
	require.Equal(t, int32(0), clicks)

	err := repo.IncrementClicks(ctx, "first")
	require.NoError(t, err)

	_, clicks, _ = repo.GetURLAndClicks(ctx, "first")
	require.Equal(t, int32(1), clicks)

	err = repo.IncrementClicks(ctx, "first")
	require.NoError(t, err)

	_, clicks, _ = repo.GetURLAndClicks(ctx, "first")
	require.Equal(t, int32(2), clicks)
}
