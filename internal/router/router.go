package router

import (
	"log/slog"
	"net/http"

	"github.com/nikitastas/subscriptions-service/internal/httpapi"
	"github.com/nikitastas/subscriptions-service/internal/subscription"
)

func New(subscriptionHandler *subscription.Handler, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.Handle("GET /swagger/", http.StripPrefix("/swagger/", http.FileServer(http.Dir("docs"))))

	subscriptionHandler.RegisterRoutes(mux)

	var handler http.Handler = mux
	handler = httpapi.Recoverer(log)(handler)
	handler = httpapi.Logger(log)(handler)
	return handler
}
