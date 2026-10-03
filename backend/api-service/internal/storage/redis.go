package storage

import (
	"context"
	"log/slog"
	"time"
	"uuid"

	"github.com/redis/go-redis/v9"
)

const (
	imageKeyPrefix        = "image:"
	imageFieldData        = "data"
	imageFieldContentType = "content_type"
)

func NewRedisClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

type ImageCache struct {
	log      *slog.Logger
	client   *redis.Client
	ttl      time.Duration
	maxBytes int
}

func NewImageCache(log *slog.Logger, client *redis.Client, ttl time.Duration, maxBytes int) *ImageCache {
	return &ImageCache{
		log:      log,
		client:   client,
		ttl:      ttl,
		maxBytes: maxBytes,
	}
}

func (c *ImageCache) Get(ctx context.Context, imageID uuid.UUID) (data []byte, contentType string, ok bool) {
	const op = "storage.ImageCache.Get"

	fields, err := c.client.HGetAll(ctx, imageKey(imageID)).Result()
	if err != nil {
		c.log.Warn("failed to get image from cache",
			slog.String("op", op),
			slog.String("image_id", imageID.String()),
			slog.String("error", err.Error()),
		)

		return nil, "", false
	}

	cachedData, hasData := fields[imageFieldData]
	if !hasData {
		return nil, "", false
	}

	return []byte(cachedData), fields[imageFieldContentType], true
}

func (c *ImageCache) Set(ctx context.Context, imageID uuid.UUID, data []byte, contentType string) {
	const op = "storage.ImageCache.Set"

	if len(data) > c.maxBytes {
		return
	}

	key := imageKey(imageID)

	if _, err := c.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, key, imageFieldData, data, imageFieldContentType, contentType)
		pipe.Expire(ctx, key, c.ttl)
		return nil
	}); err != nil {
		c.log.Warn("failed to put image into cache",
			slog.String("op", op),
			slog.String("image_id", imageID.String()),
			slog.String("error", err.Error()),
		)
	}
}

func (c *ImageCache) Delete(ctx context.Context, imageID uuid.UUID) {
	const op = "storage.ImageCache.Delete"

	if err := c.client.Del(ctx, imageKey(imageID)).Err(); err != nil {
		c.log.Warn("failed to delete image from cache",
			slog.String("op", op),
			slog.String("image_id", imageID.String()),
			slog.String("error", err.Error()),
		)
	}
}

func imageKey(imageID uuid.UUID) string {
	return imageKeyPrefix + imageID.String()
}
