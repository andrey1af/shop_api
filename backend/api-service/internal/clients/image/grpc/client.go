package grpc

import (
	"context"
	"fmt"
	"time"
	"uuid"

	"github.com/andrey1af/shop-api/backend/api-service/internal/domain"
	"github.com/andrey1af/shop-api/backend/gen/imagev1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type Client struct {
	api     imagev1.ImageServiceClient
	health  healthv1.HealthClient
	conn    *grpc.ClientConn
	timeout time.Duration
}

func New(addr string, timeout time.Duration, maxMessageBytes int) (*Client, error) {
	const op = "clients.image.grpc.New"

	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessageBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Client{
		api:     imagev1.NewImageServiceClient(conn),
		health:  healthv1.NewHealthClient(conn),
		conn:    conn,
		timeout: timeout,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Create(ctx context.Context, imageID uuid.UUID, data []byte, contentType string) error {
	const op = "clients.image.grpc.Client.Create"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if _, err := c.api.CreateImage(ctx, &imagev1.CreateImageRequest{
		Id:          imageID.String(),
		Data:        data,
		ContentType: contentType,
	}); err != nil {
		return fmt.Errorf("%s: %w", op, toAppError(err))
	}

	return nil
}

func (c *Client) Get(ctx context.Context, imageID uuid.UUID) (data []byte, contentType string, err error) {
	const op = "clients.image.grpc.Client.Get"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.api.GetImage(ctx, &imagev1.GetImageRequest{Id: imageID.String()})
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", op, toAppError(err))
	}

	return resp.GetData(), resp.GetContentType(), nil
}

func (c *Client) Replace(ctx context.Context, imageID uuid.UUID, data []byte, contentType string) error {
	const op = "clients.image.grpc.Client.Replace"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if _, err := c.api.ReplaceImage(ctx, &imagev1.ReplaceImageRequest{
		Id:          imageID.String(),
		Data:        data,
		ContentType: contentType,
	}); err != nil {
		return fmt.Errorf("%s: %w", op, toAppError(err))
	}

	return nil
}

func (c *Client) Delete(ctx context.Context, imageID uuid.UUID) error {
	const op = "clients.image.grpc.Client.Delete"

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	if _, err := c.api.DeleteImage(ctx, &imagev1.DeleteImageRequest{Id: imageID.String()}); err != nil {
		return fmt.Errorf("%s: %w", op, toAppError(err))
	}

	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	const op = "clients.image.grpc.Client.Ping"

	resp, err := c.health.Check(ctx, &healthv1.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if resp.GetStatus() != healthv1.HealthCheckResponse_SERVING {
		return fmt.Errorf("%s: image service is %s", op, resp.GetStatus())
	}

	return nil
}

func toAppError(err error) error {
	switch status.Code(err) {
	case codes.NotFound:
		return domain.ErrImageNotFound
	default:
		return err
	}
}
