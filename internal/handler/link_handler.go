package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pht/pet/link_shortener/internal/domain"
)

type LinkSaverGetter interface {
	Save(ctx context.Context, url, code string) (*domain.Link, error)
	GetURL(context.Context, string) (string, error)
	GetClicks(context.Context, string) (int32, error)
}

type LinkHandler struct {
	service LinkSaverGetter
	logger  *slog.Logger
}

func NewLinkHandler(service LinkSaverGetter, logger *slog.Logger) *LinkHandler {
	return &LinkHandler{service: service, logger: logger}
}

type codeURL struct {
	Code string `json:"code,omitempty"`
	URL  string `json:"url,omitempty"`
}

type clicksResponse struct {
	Clicks int32 `json:"clicks"`
}

func (lh *LinkHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var requestBody codeURL

	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil {
		lh.logger.Error("failed to decode the req body", "error", err.Error())
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	link, err := lh.service.Save(ctx, requestBody.URL, requestBody.Code)
	if err != nil {
		if errors.Is(err, domain.ErrNoCodeProvided) || errors.Is(err, domain.ErrNoURLProvided) {
			lh.logger.Error("bad request", "error", err.Error())
			writeError(w, http.StatusBadRequest, "bad request")
			return
		}
		lh.logger.Error("failed to save the link", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	writeJSON(w, http.StatusCreated, link)
}

func (lh *LinkHandler) GetURL(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	shortCode := r.PathValue("code")

	url, err := lh.service.GetURL(ctx, shortCode)
	if err != nil {
		if errors.Is(err, domain.ErrLinkNotFound) {
			lh.logger.Error("link not found", "error", err.Error(), "short code", shortCode)
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		lh.logger.Error("failed to get the link", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	http.Redirect(w, r, url, http.StatusFound)
}

func (lh *LinkHandler) GetClicks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	shortCode := r.PathValue("code")

	clicks, err := lh.service.GetClicks(ctx, shortCode)
	if err != nil {
		if errors.Is(err, domain.ErrLinkNotFound) {
			lh.logger.Error("link not found", "error", err.Error(), "short code", shortCode)
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		lh.logger.Error("failed to get clicks", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	resp := clicksResponse{Clicks: clicks}
	writeJSON(w, http.StatusOK, resp)
}
