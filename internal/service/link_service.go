package service

import (
	"context"
	"errors"
	"fmt"
	"pht/pet/link_shortener/internal/domain"
	"pht/pet/link_shortener/internal/domain/db"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type URLRepository interface {
	SaveLink(context.Context, db.CreateLinkParams) (*domain.Link, error)
	GetURLAndClicks(context.Context, string) (string, int32, error)
	UpdateClicks(context.Context, int32, string) error
	IncrementClicks(context.Context, string) error
}

type WarnErrorInfoLogger interface {
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Info(msg string, args ...any)
}

type LinkService struct {
	querier URLRepository
	redis   *redis.Client
	logger  WarnErrorInfoLogger
}

func NewLinkService(querier URLRepository, redisClient *redis.Client, logger WarnErrorInfoLogger) *LinkService {
	return &LinkService{querier: querier, redis: redisClient, logger: logger}
}

func (ls *LinkService) Save(ctx context.Context, url, code string) (*domain.Link, error) {
	data := domain.Link{ShortCode: code, LongUrl: url}
	if err := data.Validate(); err != nil {
		return nil, err
	}

	createLinkParams := db.CreateLinkParams{ShortCode: data.ShortCode, LongUrl: data.LongUrl}
	savedLink, err := ls.querier.SaveLink(ctx, createLinkParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create the short link for url, err: %w", err)
	}

	return savedLink, nil
}

func (ls *LinkService) GetURL(ctx context.Context, code string) (string, error) {
	redisKey := fmt.Sprintf("url:%s", code)

	redisURL, err := ls.redis.Get(ctx, redisKey).Result()
	if err == nil {
		if redisErr := ls.redis.Incr(ctx, fmt.Sprintf("clicks:%s", code)).Err(); redisErr != nil {
			ls.logger.Warn("failed to increment clicks in Redis", "error", redisErr)

			errInc := ls.querier.IncrementClicks(ctx, code)
			if errInc != nil {
				ls.logger.Error("failed to increment clicks in DB", "error", errInc)
			}
		}

		ls.logger.Info("got url from redis", "code", code)
		return redisURL, nil
	}

	if !errors.Is(err, redis.Nil) {
		ls.logger.Warn("redis error", "error", err)
	}

	url, _, err := ls.querier.GetURLAndClicks(ctx, code)
	if err != nil {
		return "", fmt.Errorf("failed to get the link: %w", err)
	}

	err = ls.redis.Set(ctx, redisKey, url, 24*time.Hour).Err()
	if err != nil {
		ls.logger.Warn("failed to cache the URL in Redis", "error", err)
	}

	if redisErr := ls.redis.Incr(ctx, fmt.Sprintf("clicks:%s", code)).Err(); redisErr != nil {
		ls.logger.Warn("failed to increment clicks in Redis", "error", redisErr)

		errInc := ls.querier.IncrementClicks(ctx, code)
		if errInc != nil {
			ls.logger.Error("failed to increment clicks in DB", "error", errInc)
		}
	}

	return url, nil
}

func (ls *LinkService) GetClicks(ctx context.Context, code string) (int32, error) {
	_, dbClicks, err := ls.querier.GetURLAndClicks(ctx, code)
	if err != nil {
		return 0, fmt.Errorf("failed to get clicks count: %w", err)
	}

	redisKey := fmt.Sprintf("clicks:%s", code)
	redisClicksStr, err := ls.redis.Get(ctx, redisKey).Result()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			ls.logger.Warn("failed to get fresh clicks from Redis. Returning DB data", "error", err)
		}
		return dbClicks, nil
	}

	redisClicks, err := strconv.ParseInt(redisClicksStr, 10, 32)
	if err != nil {
		ls.logger.Warn("failed to parse fresh clicks from Redis. Returning DB data", "error", err)
		return dbClicks, nil
	}

	clicks := dbClicks + int32(redisClicks)

	ls.logger.Info("successfully got clicks from redis and DB", "db clicks", dbClicks, "redis clicks", redisClicks)

	return clicks, nil
}

func (ls *LinkService) StartClickSyncWorker(ctx context.Context, interval time.Duration, wg *sync.WaitGroup) {
	ticker := time.NewTicker(interval)
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ls.syncClicks(ctx)
			case <-ctx.Done():
				ls.logger.Info("shutting down click sync worker, performing final sync...")

				ls.syncClicks(context.Background())
				return
			}
		}
	}()
}

func (ls *LinkService) syncClicks(ctx context.Context) {
	var cursor uint64
	for {
		keys, nextCursor, err := ls.redis.Scan(ctx, cursor, "clicks:*", 100).Result()
		if err != nil {
			ls.logger.Warn("failed to scan redis keys", "error", err)
			return
		}

		for _, key := range keys {
			code := key[7:]

			clicksStr, err := ls.redis.GetDel(ctx, key).Result()
			if errors.Is(err, redis.Nil) {
				ls.logger.Info("no data for redis key", "key", key)
				continue
			} else if err != nil {
				ls.logger.Warn("failed to GetDel redis key", "key", key, "error", err)
				continue
			}

			clicks, err := strconv.ParseInt(clicksStr, 10, 32)
			if err != nil {
				ls.logger.Warn("failed to parse clicks from Redis", "error", err)
				continue
			}

			if clicks == 0 {
				ls.logger.Info("zero clicks for redis key", "key", key)
				continue
			}

			err = ls.querier.UpdateClicks(ctx, int32(clicks), code)
			if err != nil {
				ls.logger.Warn("failed to sync clicks to DB", "code", code, "error", err)

				if incErr := ls.redis.IncrBy(ctx, key, clicks).Err(); incErr != nil {
					ls.logger.Error("failed to return clicks back to Redis. Data lost", "code", code, "clicks", clicks, "error", incErr)
				}
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
}
