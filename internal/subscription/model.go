package subscription

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var uuidRegexp = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var ErrNotFound = errors.New("subscription not found")

type Subscription struct {
	ID          string
	ServiceName string
	Price       int
	UserID      string
	StartDate   time.Time
	EndDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateRequest struct {
	ServiceName string  `json:"service_name"`
	Price       int     `json:"price"`
	UserID      string  `json:"user_id"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date,omitempty"`
}

type UpdateRequest struct {
	ServiceName string  `json:"service_name"`
	Price       int     `json:"price"`
	UserID      string  `json:"user_id"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date,omitempty"`
}

type SubscriptionResponse struct {
	ID          string  `json:"id"`
	ServiceName string  `json:"service_name"`
	Price       int     `json:"price"`
	UserID      string  `json:"user_id"`
	StartDate   string  `json:"start_date"`
	EndDate     *string `json:"end_date,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type ListFilter struct {
	UserID      string
	ServiceName string
	Limit       int
	Offset      int
}

type TotalFilter struct {
	PeriodStart time.Time
	PeriodEnd   time.Time
	UserID      string
	ServiceName string
}

type TotalResponse struct {
	TotalPrice  int64   `json:"total_price"`
	Currency    string  `json:"currency"`
	PeriodStart string  `json:"period_start"`
	PeriodEnd   string  `json:"period_end"`
	UserID      *string `json:"user_id,omitempty"`
	ServiceName *string `json:"service_name,omitempty"`
}

func NewFromCreateRequest(req CreateRequest) (Subscription, error) {
	return buildSubscription(req.ServiceName, req.Price, req.UserID, req.StartDate, req.EndDate)
}

func NewFromUpdateRequest(req UpdateRequest) (Subscription, error) {
	return buildSubscription(req.ServiceName, req.Price, req.UserID, req.StartDate, req.EndDate)
}

func ToResponse(item Subscription) SubscriptionResponse {
	var endDate *string
	if item.EndDate != nil {
		formatted := FormatMonth(*item.EndDate)
		endDate = &formatted
	}

	return SubscriptionResponse{
		ID:          item.ID,
		ServiceName: item.ServiceName,
		Price:       item.Price,
		UserID:      item.UserID,
		StartDate:   FormatMonth(item.StartDate),
		EndDate:     endDate,
		CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:   item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func ValidateUUID(value string) bool {
	return uuidRegexp.MatchString(value)
}

func buildSubscription(serviceName string, price int, userID string, startDateRaw string, endDateRaw *string) (Subscription, error) {
	serviceName = strings.TrimSpace(serviceName)
	userID = strings.TrimSpace(userID)

	if serviceName == "" {
		return Subscription{}, errors.New("service_name is required")
	}
	if len(serviceName) > 255 {
		return Subscription{}, errors.New("service_name must be at most 255 characters")
	}
	if price <= 0 {
		return Subscription{}, errors.New("price must be a positive integer")
	}
	if !ValidateUUID(userID) {
		return Subscription{}, errors.New("user_id must be a valid UUID")
	}

	startDate, err := ParseMonth(strings.TrimSpace(startDateRaw))
	if err != nil {
		return Subscription{}, fmt.Errorf("invalid start_date: %w", err)
	}

	var endDate *time.Time
	if endDateRaw != nil && strings.TrimSpace(*endDateRaw) != "" {
		parsedEndDate, err := ParseMonth(strings.TrimSpace(*endDateRaw))
		if err != nil {
			return Subscription{}, fmt.Errorf("invalid end_date: %w", err)
		}
		if parsedEndDate.Before(startDate) {
			return Subscription{}, errors.New("end_date must be greater than or equal to start_date")
		}
		endDate = &parsedEndDate
	}

	return Subscription{
		ServiceName: serviceName,
		Price:       price,
		UserID:      strings.ToLower(userID),
		StartDate:   startDate,
		EndDate:     endDate,
	}, nil
}
