package kafka

import (
	"context"
	jsonv2 "encoding/json/v2"
	"fmt"
	"time"
	"uuid"

	"github.com/segmentio/kafka-go"

	"github.com/andrey1af/shop-api/backend/generator-service/internal/domain"
)

const (
	headerEventType        = "event-type"
	eventTypeProductUpdate = "product.updated"

	batchTimeout = 10 * time.Millisecond
)

type productUpdatedMessage struct {
	EventID        uuid.UUID `json:"event_id"`
	ProductID      uuid.UUID `json:"product_id"`
	Price          float64   `json:"price"`
	AvailableStock int64     `json:"available_stock"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string, topic string, writeTimeout time.Duration) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:  kafka.TCP(brokers...),
			Topic: topic,

			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			BatchTimeout: batchTimeout,
			WriteTimeout: writeTimeout,
		},
	}
}

func (p *Producer) PublishProductUpdated(ctx context.Context, event domain.ProductUpdated) error {
	const op = "kafka.Producer.PublishProductUpdated"

	value, err := jsonv2.Marshal(productUpdatedMessage{
		EventID:        event.EventID,
		ProductID:      event.ProductID,
		Price:          event.Price,
		AvailableStock: event.AvailableStock,
		OccurredAt:     event.OccurredAt,
	})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key:     []byte(event.ProductID.String()),
		Value:   value,
		Time:    event.OccurredAt,
		Headers: []kafka.Header{{Key: headerEventType, Value: []byte(eventTypeProductUpdate)}},
	}); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
