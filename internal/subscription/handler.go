package subscription

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/nikitastas/subscriptions-service/internal/httpapi"
)

type Handler struct {
	repository *Repository
	log        *slog.Logger
}

func NewHandler(repository *Repository, log *slog.Logger) *Handler {
	return &Handler{repository: repository, log: log}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/subscriptions", h.Create)
	mux.HandleFunc("GET /api/v1/subscriptions", h.List)
	mux.HandleFunc("GET /api/v1/subscriptions/total", h.Total)
	mux.HandleFunc("GET /api/v1/subscriptions/{id}", h.Get)
	mux.HandleFunc("PUT /api/v1/subscriptions/{id}", h.Update)
	mux.HandleFunc("DELETE /api/v1/subscriptions/{id}", h.Delete)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request CreateRequest
	if err := httpapi.ReadJSON(w, r, &request); err != nil {
		h.log.Warn("create subscription: invalid request body", "error", err)
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	item, err := NewFromCreateRequest(request)
	if err != nil {
		h.log.Warn("create subscription: validation failed", "error", err)
		httpapi.WriteError(w, http.StatusBadRequest, "validation failed", err)
		return
	}

	created, err := h.repository.Create(r.Context(), item)
	if err != nil {
		h.log.Error("create subscription: repository error", "error", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to create subscription", nil)
		return
	}

	httpapi.WriteJSON(w, http.StatusCreated, ToResponse(created))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !ValidateUUID(id) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid subscription id", fmt.Errorf("id must be a valid UUID"))
		return
	}

	item, err := h.repository.GetByID(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httpapi.WriteError(w, http.StatusNotFound, "subscription not found", nil)
		return
	}
	if err != nil {
		h.log.Error("get subscription: repository error", "id", id, "error", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to get subscription", nil)
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, ToResponse(item))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, err := parseIntQuery(query.Get("limit"), 50, 1, 100)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid limit", err)
		return
	}
	offset, err := parseIntQuery(query.Get("offset"), 0, 0, 1_000_000)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid offset", err)
		return
	}

	userID := strings.TrimSpace(query.Get("user_id"))
	if userID != "" && !ValidateUUID(userID) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid user_id", fmt.Errorf("user_id must be a valid UUID"))
		return
	}

	filter := ListFilter{
		UserID:      strings.ToLower(userID),
		ServiceName: strings.TrimSpace(query.Get("service_name")),
		Limit:       limit,
		Offset:      offset,
	}

	items, err := h.repository.List(r.Context(), filter)
	if err != nil {
		h.log.Error("list subscriptions: repository error", "error", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list subscriptions", nil)
		return
	}

	responseItems := make([]SubscriptionResponse, 0, len(items))
	for _, item := range items {
		responseItems = append(responseItems, ToResponse(item))
	}

	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"items":  responseItems,
		"count":  len(responseItems),
		"limit":  limit,
		"offset": offset,
	})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !ValidateUUID(id) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid subscription id", fmt.Errorf("id must be a valid UUID"))
		return
	}

	var request UpdateRequest
	if err := httpapi.ReadJSON(w, r, &request); err != nil {
		h.log.Warn("update subscription: invalid request body", "id", id, "error", err)
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	item, err := NewFromUpdateRequest(request)
	if err != nil {
		h.log.Warn("update subscription: validation failed", "id", id, "error", err)
		httpapi.WriteError(w, http.StatusBadRequest, "validation failed", err)
		return
	}

	updated, err := h.repository.Update(r.Context(), id, item)
	if errors.Is(err, ErrNotFound) {
		httpapi.WriteError(w, http.StatusNotFound, "subscription not found", nil)
		return
	}
	if err != nil {
		h.log.Error("update subscription: repository error", "id", id, "error", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to update subscription", nil)
		return
	}

	httpapi.WriteJSON(w, http.StatusOK, ToResponse(updated))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !ValidateUUID(id) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid subscription id", fmt.Errorf("id must be a valid UUID"))
		return
	}

	if err := h.repository.Delete(r.Context(), id); errors.Is(err, ErrNotFound) {
		httpapi.WriteError(w, http.StatusNotFound, "subscription not found", nil)
		return
	} else if err != nil {
		h.log.Error("delete subscription: repository error", "id", id, "error", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to delete subscription", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Total(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	periodStartRaw := strings.TrimSpace(query.Get("period_start"))
	periodEndRaw := strings.TrimSpace(query.Get("period_end"))
	if periodStartRaw == "" || periodEndRaw == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "validation failed", fmt.Errorf("period_start and period_end are required"))
		return
	}

	periodStart, err := ParseMonth(periodStartRaw)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid period_start", err)
		return
	}
	periodEnd, err := ParseMonth(periodEndRaw)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid period_end", err)
		return
	}
	if periodEnd.Before(periodStart) {
		httpapi.WriteError(w, http.StatusBadRequest, "validation failed", fmt.Errorf("period_end must be greater than or equal to period_start"))
		return
	}

	userID := strings.TrimSpace(query.Get("user_id"))
	if userID != "" && !ValidateUUID(userID) {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid user_id", fmt.Errorf("user_id must be a valid UUID"))
		return
	}

	serviceName := strings.TrimSpace(query.Get("service_name"))
	filter := TotalFilter{
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		UserID:      strings.ToLower(userID),
		ServiceName: serviceName,
	}

	total, err := h.repository.CalculateTotal(r.Context(), filter)
	if err != nil {
		h.log.Error("calculate total: repository error", "error", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to calculate total", nil)
		return
	}

	response := TotalResponse{
		TotalPrice:  total,
		Currency:    "RUB",
		PeriodStart: FormatMonth(periodStart),
		PeriodEnd:   FormatMonth(periodEnd),
	}
	if userID != "" {
		response.UserID = &filter.UserID
	}
	if serviceName != "" {
		response.ServiceName = &filter.ServiceName
	}

	httpapi.WriteJSON(w, http.StatusOK, response)
}

func parseIntQuery(value string, defaultValue, minValue, maxValue int) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("must be an integer")
	}
	if parsed < minValue || parsed > maxValue {
		return 0, fmt.Errorf("must be between %d and %d", minValue, maxValue)
	}
	return parsed, nil
}
