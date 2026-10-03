package broadcast

import (
	"log/slog"
	"sync"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
)

const subscriberBuffer = 64

type ProductHub struct {
	log *slog.Logger

	mu          sync.Mutex
	subscribers map[chan domain.Product]struct{}
	closed      bool
}

func NewProductHub(log *slog.Logger) *ProductHub {
	return &ProductHub{
		log:         log,
		subscribers: make(map[chan domain.Product]struct{}),
	}
}

func (h *ProductHub) Subscribe() (<-chan domain.Product, func()) {
	ch := make(chan domain.Product, subscriberBuffer)

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		close(ch)
		return ch, func() {}
	}

	h.subscribers[ch] = struct{}{}

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()

			if _, ok := h.subscribers[ch]; ok {
				delete(h.subscribers, ch)
				close(ch)
			}
		})
	}

	return ch, unsubscribe
}

func (h *ProductHub) NotifyProductUpdated(product domain.Product) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.subscribers {
		select {
		case ch <- product:
		default:
			h.log.Warn("product update dropped for slow subscriber",
				slog.String("op", "broadcast.ProductHub.NotifyProductUpdated"),
				slog.String("product_id", product.ID.String()),
			)
		}
	}
}

func (h *ProductHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return
	}
	h.closed = true

	for ch := range h.subscribers {
		delete(h.subscribers, ch)
		close(ch)
	}
}
