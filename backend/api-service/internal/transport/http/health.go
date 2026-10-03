package http

import (
	"context"
	"net/http"
	"sync"
	"time"
)

const (
	healthStatusOK   = "ok"
	healthStatusFail = "fail"
)

const (
	healthCheckDatabase = "database"
	healthCheckStorage  = "storage"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type livenessResponse struct {
	Status string `json:"status"`
}

type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

type HealthHandler struct {
	timeout  time.Duration
	database Pinger
	storage  Pinger
}

func NewHealthHandler(timeout time.Duration, database, storage Pinger) *HealthHandler {
	return &HealthHandler{
		timeout:  timeout,
		database: database,
		storage:  storage,
	}
}

func (h *HealthHandler) Liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, livenessResponse{Status: healthStatusOK})
}

func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	targets := []struct {
		name   string
		pinger Pinger
	}{
		{name: healthCheckDatabase, pinger: h.database},
		{name: healthCheckStorage, pinger: h.storage},
	}

	errs := make([]error, len(targets))

	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = target.pinger.Ping(ctx)
		}()
	}
	wg.Wait()

	status := healthStatusOK
	statusCode := http.StatusOK
	checks := make(map[string]string, len(targets))

	for i, target := range targets {
		if errs[i] != nil {
			checks[target.name] = healthStatusFail
			status = healthStatusFail
			statusCode = http.StatusServiceUnavailable
			continue
		}
		checks[target.name] = healthStatusOK
	}

	writeJSON(w, statusCode, readinessResponse{Status: status, Checks: checks})
}
