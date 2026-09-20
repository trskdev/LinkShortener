-- name: CreateLink :one
INSERT INTO links (short_code, long_url) VALUES ($1, $2) RETURNING id, short_code, long_url, clicks;

-- name: GetURLAndClicksByCode :one
SELECT long_url, clicks FROM links WHERE short_code = $1 LIMIT 1;

-- name: UpdateClicks :exec
UPDATE links SET clicks = clicks + $2 WHERE short_code = $1;

-- name: IncrementClicks :exec
UPDATE links SET clicks = clicks + 1 WHERE short_code = $1;