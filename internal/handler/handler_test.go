//go:build integration

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pht/pet/link_shortener/internal/domain"
	"pht/pet/link_shortener/internal/mocks"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockSetupFunc func(context.Context, *mocks.MockLinkSaverGetter)

type response struct {
	status int
	body   any
}

func TestLinkHandler_Create(t *testing.T) {
	validLink := &domain.Link{ID: 1, ShortCode: "test", LongUrl: "https://test.com", Clicks: 0}
	errUnknown := errors.New("failed to save the link")

	cases := []struct {
		name            string
		args            any
		willingResponse response
		mockSetup       mockSetupFunc
	}{
		{
			name:            "successful",
			args:            codeURL{Code: "test", URL: "https://test.com"},
			willingResponse: response{status: http.StatusCreated, body: validLink},
			mockSetup: func(ctx context.Context, m *mocks.MockLinkSaverGetter) {
				m.EXPECT().Save(ctx, mock.Anything, mock.Anything).Return(validLink, nil)
			},
		},
		{
			name:            "failed to decode json",
			args:            `{"code":`,
			willingResponse: response{status: http.StatusBadRequest, body: apiError{Error: "bad request"}},
		},
		{
			name:            "no url provided",
			args:            codeURL{Code: "test"},
			willingResponse: response{status: http.StatusBadRequest, body: apiError{Error: "bad request"}},
			mockSetup: func(ctx context.Context, m *mocks.MockLinkSaverGetter) {
				m.EXPECT().Save(ctx, mock.Anything, mock.Anything).Return(nil, domain.ErrNoURLProvided)
			},
		},
		{
			name:            "no code provided",
			args:            codeURL{URL: "https://test.com"},
			willingResponse: response{status: http.StatusBadRequest, body: apiError{Error: "bad request"}},
			mockSetup: func(ctx context.Context, m *mocks.MockLinkSaverGetter) {
				m.EXPECT().Save(ctx, mock.Anything, mock.Anything).Return(nil, domain.ErrNoCodeProvided)
			},
		},
		{
			name:            "save method fail",
			args:            codeURL{Code: "test", URL: "https://test.com"},
			willingResponse: response{status: http.StatusInternalServerError, body: apiError{Error: "server error"}},
			mockSetup: func(ctx context.Context, m *mocks.MockLinkSaverGetter) {
				m.EXPECT().Save(ctx, mock.Anything, mock.Anything).Return(nil, errUnknown)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serviceMock := mocks.NewMockLinkSaverGetter(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			linkHandler := NewLinkHandler(serviceMock, logger)

			jsonData, err := json.Marshal(tc.args)
			if err != nil {
				require.NoError(t, err)
			}
			bodyReader := bytes.NewBuffer(jsonData)

			req := httptest.NewRequest(http.MethodPost, "/shorten", bodyReader)
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()

			if tc.mockSetup != nil {
				tc.mockSetup(req.Context(), serviceMock)
			}

			mux := http.NewServeMux()
			mux.Handle("POST /shorten", http.HandlerFunc(linkHandler.Create))

			mux.ServeHTTP(w, req)

			assert.Equal(t, tc.willingResponse.status, w.Code)

			if tc.willingResponse.body != nil {
				expJSON, err := json.Marshal(tc.willingResponse.body)
				assert.NoError(t, err)

				assert.JSONEq(t, string(expJSON), w.Body.String())
			}
		})
	}
}

func TestLinkHandler_GetURL(t *testing.T) {
	errUnknown := errors.New("failed to get the url")

	cases := []struct {
		name            string
		path            string
		url             string
		willingResponse response
		mockSetup       mockSetupFunc
	}{
		{
			name:            "successful",
			path:            "/test",
			url:             "https://test.com",
			willingResponse: response{status: http.StatusFound},
			mockSetup: func(ctx context.Context, m *mocks.MockLinkSaverGetter) {
				m.EXPECT().GetURL(ctx, "test").Return("https://test.com", nil)
			},
		},
		{
			name:            "no code provided, 404",
			path:            "/",
			willingResponse: response{status: http.StatusNotFound},
		},
		{
			name:            "link not found, 404",
			path:            "/test",
			willingResponse: response{status: http.StatusNotFound, body: apiError{Error: "not found"}},
			mockSetup: func(ctx context.Context, m *mocks.MockLinkSaverGetter) {
				m.EXPECT().GetURL(ctx, mock.Anything).Return("", domain.ErrLinkNotFound)
			},
		},
		{
			name:            "get method fail",
			path:            "/test",
			willingResponse: response{status: http.StatusInternalServerError, body: apiError{Error: "server error"}},
			mockSetup: func(ctx context.Context, m *mocks.MockLinkSaverGetter) {
				m.EXPECT().GetURL(ctx, mock.Anything).Return("", errUnknown)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serviceMock := mocks.NewMockLinkSaverGetter(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			linkHandler := NewLinkHandler(serviceMock, logger)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()

			if tc.mockSetup != nil {
				tc.mockSetup(req.Context(), serviceMock)
			}

			mux := http.NewServeMux()
			mux.Handle("GET /{code}", http.HandlerFunc(linkHandler.GetURL))

			mux.ServeHTTP(w, req)

			assert.Equal(t, tc.willingResponse.status, w.Code)

			if tc.url != "" {
				assert.Equal(t, tc.url, w.Header().Get("Location"))
			}

			if tc.willingResponse.body != nil {
				expJSON, err := json.Marshal(tc.willingResponse.body)
				assert.NoError(t, err)

				assert.JSONEq(t, string(expJSON), w.Body.String())
			}
		})
	}
}

func TestLinkHandler_GetClicks(t *testing.T) {
	errUnknown := errors.New("failed to get clicks")

	cases := []struct {
		name            string
		path            string
		willingResponse response
		mockSetup       mockSetupFunc
	}{
		{
			name:            "successful",
			path:            "/stats/test",
			willingResponse: response{status: http.StatusOK, body: clicksResponse{Clicks: 16}},
			mockSetup: func(ctx context.Context, m *mocks.MockLinkSaverGetter) {
				m.EXPECT().GetClicks(ctx, "test").Return(16, nil)
			},
		},
		{
			name:            "no code provided, 404",
			path:            "/stats/",
			willingResponse: response{status: http.StatusNotFound},
		},
		{
			name:            "link not found, 404",
			path:            "/stats/test",
			willingResponse: response{status: http.StatusNotFound, body: apiError{Error: "not found"}},
			mockSetup: func(ctx context.Context, m *mocks.MockLinkSaverGetter) {
				m.EXPECT().GetClicks(ctx, mock.Anything).Return(0, domain.ErrLinkNotFound)
			},
		},
		{
			name:            "get method fail",
			path:            "/stats/test",
			willingResponse: response{status: http.StatusInternalServerError, body: apiError{Error: "server error"}},
			mockSetup: func(ctx context.Context, m *mocks.MockLinkSaverGetter) {
				m.EXPECT().GetClicks(ctx, mock.Anything).Return(0, errUnknown)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serviceMock := mocks.NewMockLinkSaverGetter(t)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			linkHandler := NewLinkHandler(serviceMock, logger)

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()

			if tc.mockSetup != nil {
				tc.mockSetup(req.Context(), serviceMock)
			}

			mux := http.NewServeMux()
			mux.Handle("GET /stats/{code}", http.HandlerFunc(linkHandler.GetClicks))

			mux.ServeHTTP(w, req)

			assert.Equal(t, tc.willingResponse.status, w.Code)

			if tc.willingResponse.body != nil {
				expJSON, err := json.Marshal(tc.willingResponse.body)
				assert.NoError(t, err)

				assert.JSONEq(t, string(expJSON), w.Body.String())
			}
		})
	}
}
