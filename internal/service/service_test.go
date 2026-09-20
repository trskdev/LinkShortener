package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"pht/pet/link_shortener/internal/app"
	"pht/pet/link_shortener/internal/domain"
	"pht/pet/link_shortener/internal/mocks"
	"pht/pet/link_shortener/internal/util"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockSetupFunc func(context.Context, *mocks.MockURLRepository)

type redisSetupFunc func(redismock.ClientMock)

type expectedLog struct {
	level   slog.Level
	message string
	err     error
}

func TestLinkService_Save(t *testing.T) {
	ctx := context.Background()

	validLink := &domain.Link{ID: 1, ShortCode: "test", LongUrl: "https://test.com", Clicks: 0}
	errUnknown := errors.New("failed to save the link")

	cases := []struct {
		name        string
		code        string
		url         string
		expectedErr error
		mockSetup   mockSetupFunc
	}{
		{
			name: "successful",
			code: "test",
			url:  "https://test.com",
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().SaveLink(ctx, mock.Anything).Return(validLink, nil)
			},
		},
		{
			name:        "no url err",
			code:        "test",
			url:         "",
			expectedErr: domain.ErrNoURLProvided,
		},
		{
			name:        "no code err",
			code:        "",
			url:         "https://test.com",
			expectedErr: domain.ErrNoCodeProvided,
		},
		{
			name:        "nothing provided",
			code:        "",
			url:         "",
			expectedErr: domain.ErrNoURLProvided,
		},
		{
			name:        "repo call err",
			code:        "test",
			url:         "https://test.com",
			expectedErr: errUnknown,
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().SaveLink(ctx, mock.Anything).Return(nil, errUnknown)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repoMock := mocks.NewMockURLRepository(t)
			redisClient, redisMock := redismock.NewClientMock()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			linkService := NewLinkService(repoMock, redisClient, logger)

			if tc.mockSetup != nil {
				tc.mockSetup(ctx, repoMock)
			}

			link, err := linkService.Save(ctx, tc.url, tc.code)

			if tc.expectedErr == nil {
				require.NoError(t, err)
				assert.Equal(t, validLink, link)
				assert.NoError(t, redisMock.ExpectationsWereMet())
				return
			}

			assert.ErrorIs(t, err, tc.expectedErr)
			assert.Nil(t, link)
			assert.NoError(t, redisMock.ExpectationsWereMet())
		})
	}
}

func TestLinkService_GetURL(t *testing.T) {
	ctx := context.Background()

	errUnknown := errors.New("failed to get the url")
	errShouldFail := errors.New("this call is meant to fail")

	cases := []struct {
		name        string
		code        string
		url         string
		expectedErr error
		mockSetup   mockSetupFunc
		redisSetup  redisSetupFunc
		expLogs     []expectedLog
	}{
		{
			name: "two successful redis calls",
			code: "test",
			url:  "https://test.com",
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectGet("url:test").SetVal("https://test.com")
				r.ExpectIncr("clicks:test").SetVal(1)
			},
			expLogs: []expectedLog{
				{level: slog.LevelInfo, message: "got url from redis"},
			},
		},
		{
			name: "url from redis, clicks by direct call to repo",
			code: "test",
			url:  "https://test.com",
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().IncrementClicks(ctx, "test").Return(nil)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectGet("url:test").SetVal("https://test.com")
				r.ExpectIncr("clicks:test").SetErr(errShouldFail)
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "failed to increment clicks in Redis", err: errShouldFail},
				{level: slog.LevelInfo, message: "got url from redis"},
			},
		},
		{
			name: "url from redis, clicks increment failed both by redis and repo call",
			code: "test",
			url:  "https://test.com",
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().IncrementClicks(ctx, "test").Return(errShouldFail)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectGet("url:test").SetVal("https://test.com")
				r.ExpectIncr("clicks:test").SetErr(errShouldFail)
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "failed to increment clicks in Redis", err: errShouldFail},
				{level: slog.LevelError, message: "failed to increment clicks in DB", err: errShouldFail},
				{level: slog.LevelInfo, message: "got url from redis"},
			},
		},
		{
			name: "url redis miss, success",
			code: "test",
			url:  "https://test.com",
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().GetURLAndClicks(ctx, "test").Return("https://test.com", int32(0), nil)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectGet("url:test").SetErr(redis.Nil)
				r.ExpectSet("url:test", "https://test.com", 24*time.Hour).SetVal("OK")
				r.ExpectIncr("clicks:test").SetVal(1)
			},
		},
		{
			name: "url redis get, set, incr err, repo.Increment err, total success",
			code: "test",
			url:  "https://test.com",
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().GetURLAndClicks(ctx, "test").Return("https://test.com", int32(0), nil)
				m.EXPECT().IncrementClicks(ctx, "test").Return(errShouldFail)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectGet("url:test").SetErr(errShouldFail)
				r.ExpectSet("url:test", "https://test.com", 24*time.Hour).SetErr(errShouldFail)
				r.ExpectIncr("clicks:test").SetErr(errShouldFail)
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "redis error", err: errShouldFail},
				{level: slog.LevelWarn, message: "failed to cache the URL in Redis", err: errShouldFail},
				{level: slog.LevelWarn, message: "failed to increment clicks in Redis", err: errShouldFail},
				{level: slog.LevelError, message: "failed to increment clicks in DB", err: errShouldFail},
			},
		},
		{
			name:        "url redis get, set, incr err, repo.Increment err, total success",
			code:        "test",
			url:         "https://test.com",
			expectedErr: errUnknown,
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().GetURLAndClicks(ctx, "test").Return("", int32(0), errUnknown)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectGet("url:test").SetErr(errShouldFail)
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "redis error", err: errShouldFail},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collector := &app.LogCollector{}
			testLogger := slog.New(collector)

			repoMock := mocks.NewMockURLRepository(t)
			redisClient, redisMock := redismock.NewClientMock()

			linkService := NewLinkService(repoMock, redisClient, testLogger)

			if tc.mockSetup != nil {
				tc.mockSetup(ctx, repoMock)
			}

			if tc.redisSetup != nil {
				tc.redisSetup(redisMock)
			}

			url, err := linkService.GetURL(ctx, tc.code)

			require.Len(t, collector.Records, len(tc.expLogs))
			if tc.expLogs != nil {
				for i, exp := range tc.expLogs {
					assert.Equal(t, exp.level, collector.Records[i].Level)
					assert.Equal(t, exp.message, collector.Records[i].Message)

					if exp.err != nil {
						loggedErr, found := util.GetErrFromLog(collector.Records[i])
						assert.True(t, found)
						assert.ErrorIs(t, loggedErr, exp.err)
					}
				}
			}

			if tc.expectedErr == nil {
				require.NoError(t, err)
				assert.Equal(t, tc.url, url)
				assert.NoError(t, redisMock.ExpectationsWereMet())
				return
			}

			assert.ErrorIs(t, err, tc.expectedErr)
			assert.Equal(t, "", url)

			assert.NoError(t, redisMock.ExpectationsWereMet())
		})
	}
}

func TestLinkService_GetClicks(t *testing.T) {
	ctx := context.Background()

	errUnknown := errors.New("failed to get clicks")
	errShouldFail := errors.New("this call is meant to fail")

	cases := []struct {
		name        string
		code        string
		clicks      int32
		expectedErr error
		mockSetup   mockSetupFunc
		redisSetup  redisSetupFunc
		expLogs     []expectedLog
	}{
		{
			name:   "successful DB and redis calls",
			code:   "test",
			clicks: 6,
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().GetURLAndClicks(ctx, "test").Return(mock.Anything, int32(4), nil)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectGet("clicks:test").SetVal("2")
			},
			expLogs: []expectedLog{
				{level: slog.LevelInfo, message: "successfully got clicks from redis and DB"},
			},
		},
		{
			name:   "successful DB call and unparsable redis",
			code:   "test",
			clicks: 4,
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().GetURLAndClicks(ctx, "test").Return(mock.Anything, int32(4), nil)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectGet("clicks:test").SetVal("unparsable")
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "failed to parse fresh clicks from Redis. Returning DB data", err: strconv.ErrSyntax},
			},
		},
		{
			name:   "successful DB call and redis miss",
			code:   "test",
			clicks: 4,
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().GetURLAndClicks(ctx, "test").Return(mock.Anything, int32(4), nil)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectGet("clicks:test").SetErr(redis.Nil)
			},
		},
		{
			name:   "successful DB call and redis err",
			code:   "test",
			clicks: 4,
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().GetURLAndClicks(ctx, "test").Return(mock.Anything, int32(4), nil)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectGet("clicks:test").SetErr(errShouldFail)
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "failed to get fresh clicks from Redis. Returning DB data", err: errShouldFail},
			},
		},
		{
			name:        "successful DB call and redis err",
			code:        "test",
			clicks:      0,
			expectedErr: errUnknown,
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().GetURLAndClicks(ctx, "test").Return(mock.Anything, int32(4), errUnknown)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collector := &app.LogCollector{}
			testLogger := slog.New(collector)

			repoMock := mocks.NewMockURLRepository(t)
			redisClient, redisMock := redismock.NewClientMock()

			linkService := NewLinkService(repoMock, redisClient, testLogger)

			if tc.mockSetup != nil {
				tc.mockSetup(ctx, repoMock)
			}

			if tc.redisSetup != nil {
				tc.redisSetup(redisMock)
			}

			clicks, err := linkService.GetClicks(ctx, tc.code)

			require.Len(t, collector.Records, len(tc.expLogs))
			if tc.expLogs != nil {
				for i, exp := range tc.expLogs {
					assert.Equal(t, exp.level, collector.Records[i].Level)
					assert.Equal(t, exp.message, collector.Records[i].Message)

					if exp.err != nil {
						loggedErr, found := util.GetErrFromLog(collector.Records[i])
						assert.True(t, found)
						assert.ErrorIs(t, loggedErr, exp.err)
					}
				}
			}

			if tc.expectedErr == nil {
				require.NoError(t, err)
				assert.Equal(t, tc.clicks, clicks)
				assert.NoError(t, redisMock.ExpectationsWereMet())
				return
			}

			assert.ErrorIs(t, err, tc.expectedErr)
			assert.Equal(t, int32(0), clicks)

			assert.NoError(t, redisMock.ExpectationsWereMet())
		})
	}
}

func TestLinkService_SyncClicks(t *testing.T) {
	ctx := context.Background()

	errShouldFail := errors.New("this call is meant to fail")

	cases := []struct {
		name       string
		mockSetup  mockSetupFunc
		redisSetup redisSetupFunc
		expLogs    []expectedLog
	}{
		{
			name: "success",
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().UpdateClicks(ctx, mock.Anything, "test").Return(nil)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectScan(0, "clicks:*", 100).SetVal([]string{"clicks:test"}, 0)
				r.ExpectGetDel("clicks:test").SetVal("16")
			},
		},
		{
			name: "failed to update clicks count in DB",
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().UpdateClicks(ctx, mock.Anything, "test").Return(errShouldFail)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectScan(0, "clicks:*", 100).SetVal([]string{"clicks:test"}, 0)
				r.ExpectGetDel("clicks:test").SetVal("16")
				r.ExpectIncrBy("clicks:test", int64(16)).SetVal(16)
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "failed to sync clicks to DB", err: errShouldFail},
			},
		},
		{
			name: "failed to update clicks count in DB and to return them to redis",
			mockSetup: func(ctx context.Context, m *mocks.MockURLRepository) {
				m.EXPECT().UpdateClicks(ctx, mock.Anything, "test").Return(errShouldFail)
			},
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectScan(0, "clicks:*", 100).SetVal([]string{"clicks:test"}, 0)
				r.ExpectGetDel("clicks:test").SetVal("16")
				r.ExpectIncrBy("clicks:test", int64(16)).SetErr(errShouldFail)
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "failed to sync clicks to DB", err: errShouldFail},
				{level: slog.LevelError, message: "failed to return clicks back to Redis. Data lost", err: errShouldFail},
			},
		},
		{
			name: "got zero clicks from redis",
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectScan(0, "clicks:*", 100).SetVal([]string{"clicks:test"}, 0)
				r.ExpectGetDel("clicks:test").SetVal("0")
			},
			expLogs: []expectedLog{
				{level: slog.LevelInfo, message: "zero clicks for redis key"},
			},
		},
		{
			name: "failed to parse clicks from redis",
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectScan(0, "clicks:*", 100).SetVal([]string{"clicks:test"}, 0)
				r.ExpectGetDel("clicks:test").SetVal("zero")
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "failed to parse clicks from Redis", err: strconv.ErrSyntax},
			},
		},
		{
			name: "no data from redis",
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectScan(0, "clicks:*", 100).SetVal([]string{"clicks:test"}, 0)
				r.ExpectGetDel("clicks:test").SetErr(redis.Nil)
			},
			expLogs: []expectedLog{
				{level: slog.LevelInfo, message: "no data for redis key"},
			},
		},
		{
			name: "failed to getdel from redis",
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectScan(0, "clicks:*", 100).SetVal([]string{"clicks:test"}, 0)
				r.ExpectGetDel("clicks:test").SetErr(errShouldFail)
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "failed to GetDel redis key", err: errShouldFail},
			},
		},
		{
			name: "failed to scan redis keys",
			redisSetup: func(r redismock.ClientMock) {
				r.ExpectScan(0, "clicks:*", 100).SetErr(errShouldFail)
			},
			expLogs: []expectedLog{
				{level: slog.LevelWarn, message: "failed to scan redis keys", err: errShouldFail},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			collector := &app.LogCollector{}
			testLogger := slog.New(collector)

			repoMock := mocks.NewMockURLRepository(t)
			redisClient, redisMock := redismock.NewClientMock()

			linkService := NewLinkService(repoMock, redisClient, testLogger)

			if tc.mockSetup != nil {
				tc.mockSetup(ctx, repoMock)
			}

			if tc.redisSetup != nil {
				tc.redisSetup(redisMock)
			}

			linkService.syncClicks(ctx)

			require.Len(t, collector.Records, len(tc.expLogs))
			if tc.expLogs != nil {
				for i, exp := range tc.expLogs {
					assert.Equal(t, exp.level, collector.Records[i].Level)
					assert.Equal(t, exp.message, collector.Records[i].Message)

					if exp.err != nil {
						loggedErr, found := util.GetErrFromLog(collector.Records[i])
						assert.True(t, found)
						assert.ErrorIs(t, loggedErr, exp.err)
					}
				}
			}

			assert.NoError(t, redisMock.ExpectationsWereMet())
		})
	}
}

func TestLinkService_StartClickSyncWorker_Shutdown(t *testing.T) {
	collector := &app.LogCollector{}
	testLogger := slog.New(collector)
	repoMock := mocks.NewMockURLRepository(t)
	redisClient, redisMock := redismock.NewClientMock()

	linkService := NewLinkService(repoMock, redisClient, testLogger)

	redisMock.ExpectScan(0, "clicks:*", 100).SetVal([]string{}, 0)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	linkService.StartClickSyncWorker(ctx, time.Hour, &wg)

	time.Sleep(time.Millisecond * 5)
	cancel()

	wg.Wait()

	require.Len(t, collector.Records, 1)
	assert.Equal(t, slog.LevelInfo, collector.Records[0].Level)
	assert.Equal(t, "shutting down click sync worker, performing final sync...", collector.Records[0].Message)

	assert.NoError(t, redisMock.ExpectationsWereMet())
}
