package handler

import (
	"net/http"

	"github.com/initify/flakie/pkg/app"
)

// Handler is the Vercel serverless function entrypoint for GitHub webhooks.
func Handler(w http.ResponseWriter, r *http.Request) {
	router, err := app.RouterFromEnv()
	if err != nil {
		http.Error(w, "config error", http.StatusInternalServerError)
		return
	}
	// Delegate to the shared Gin router.
	router.ServeHTTP(w, r)
}
