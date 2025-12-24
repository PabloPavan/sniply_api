package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type HealthHandler struct {
	DB *pgxpool.Pool
}

func (h *HealthHandler) Get(w http.ResponseWriter, r *http.Request) {
	type resp struct {
		Status string `json:"status"`
		DB     string `json:"db"`
		Time   string `json:"time"`
	}

	dbStatus := "ok"
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.DB.Ping(ctx); err != nil {
		dbStatus = "down"
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp{
		Status: "ok",
		DB:     dbStatus,
		Time:   time.Now().UTC().Format(time.RFC3339),
	})
}
