package http

import (
	jsonv2 "encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
)

const (
	sseEventProductUpdated = "product.updated"

	sseHeartbeatInterval = 15 * time.Second

	sseRetryMillis = 3000
)

type productUpdateSubscriber interface {
	Subscribe() (<-chan domain.Product, func())
}

type ProductStreamHandler struct {
	subscriber        productUpdateSubscriber
	heartbeatInterval time.Duration
}

func NewProductStreamHandler(subscriber productUpdateSubscriber) *ProductStreamHandler {
	return &ProductStreamHandler{
		subscriber:        subscriber,
		heartbeatInterval: sseHeartbeatInterval,
	}
}

func (h *ProductStreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
	controller := http.NewResponseController(w)

	updates, unsubscribe := h.subscriber.Subscribe()
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprintf(w, "retry: %d\n\n", sseRetryMillis); err != nil {
		return
	}
	if err := controller.Flush(); err != nil {
		return
	}

	heartbeat := time.NewTicker(h.heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case product, ok := <-updates:
			if !ok {
				return
			}
			if err := writeSSEEvent(w, sseEventProductUpdated, toProductResponse(product)); err != nil {
				return
			}

		case <-heartbeat.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
		}

		if err := controller.Flush(); err != nil {
			return
		}
	}
}

func writeSSEEvent(w io.Writer, event string, data any) error {
	payload, err := jsonv2.Marshal(data)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)

	return err
}
