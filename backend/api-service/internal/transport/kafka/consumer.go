package kafka

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/segmentio/kafka-go"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
	apperrors "github.com/andrey1af/shop-api/backend/api-service/internal/errors"
	"github.com/andrey1af/shop-api/backend/api-service/internal/usecase"
)

const (
	retryInitialBackoff = 500 * time.Millisecond
	retryMaxBackoff     = 30 * time.Second
)

var errMalformedMessage = errors.New("malformed message")

type productUseCase interface {
	ApplyUpdate(ctx context.Context, in usecase.ApplyProductUpdateInput) (domain.Product, error)
}

type productUpdatedMessage struct {
	EventID        uuid.UUID `json:"event_id"`
	ProductID      uuid.UUID `json:"product_id"`
	Price          float64   `json:"price"`
	AvailableStock int64     `json:"available_stock"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type Consumer struct {
	log            *slog.Logger
	reader         *kafka.Reader
	productUseCase productUseCase
}

func NewConsumer(log *slog.Logger, brokers []string, topic, groupID string, productUseCase productUseCase) *Consumer {
	return &Consumer{
		log: log,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     brokers,
			Topic:       topic,
			GroupID:     groupID,
			StartOffset: kafka.FirstOffset,
		}),
		productUseCase: productUseCase,
	}
}

func (c *Consumer) Run(ctx context.Context) {
	const op = "kafka.Consumer.Run"
	log := c.log.With(slog.String("op", op), slog.String("topic", c.reader.Config().Topic))

	log.Info("kafka consumer started")

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Info("kafka consumer stopped")
				return
			}

			log.Error("failed to fetch message", slog.String("error", err.Error()))
			continue
		}

		if !c.handleWithRetry(ctx, msg) {
			log.Info("kafka consumer stopped")
			return
		}

		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				log.Info("kafka consumer stopped")
				return
			}

			log.Error("failed to commit message", slog.Int64("offset", msg.Offset), slog.String("error", err.Error()))
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}

func (c *Consumer) handleWithRetry(ctx context.Context, msg kafka.Message) bool {
	log := c.log.With(
		slog.String("op", "kafka.Consumer.handle"),
		slog.Int("partition", msg.Partition),
		slog.Int64("offset", msg.Offset),
	)

	backoff := retryInitialBackoff

	for {
		err := c.handle(ctx, msg)
		if err == nil {
			return true
		}

		if isPermanent(err) {
			log.Warn("skip message", slog.String("error", err.Error()))
			return true
		}

		log.Error("failed to handle message, retrying",
			slog.Duration("backoff", backoff),
			slog.String("error", err.Error()),
		)

		select {
		case <-ctx.Done():
			return false
		case <-time.After(backoff):
		}

		backoff = min(backoff*2, retryMaxBackoff)
	}
}

func (c *Consumer) handle(ctx context.Context, msg kafka.Message) error {
	var event productUpdatedMessage
	if err := jsonv2.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("%w: %w", errMalformedMessage, err)
	}

	product, err := c.productUseCase.ApplyUpdate(ctx, usecase.ApplyProductUpdateInput{
		ProductID:      event.ProductID,
		Price:          event.Price,
		AvailableStock: event.AvailableStock,
	})
	if err != nil {
		return fmt.Errorf("event %s: %w", event.EventID, err)
	}

	c.log.Info("product update applied",
		slog.String("event_id", event.EventID.String()),
		slog.String("product_id", product.ID.String()),
		slog.Float64("price", product.Price),
		slog.Int64("available_stock", product.AvailableStock),
	)

	return nil
}

func isPermanent(err error) bool {
	return errors.Is(err, errMalformedMessage) ||
		errors.Is(err, apperrors.ErrInvalidProductUpdate) ||
		errors.Is(err, domain.ErrProductNotFound)
}
